package install

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// SetupResult reports what the setup wizard did and what the user still
// needs to do manually (only the extension-load step can never be automated).
type SetupResult struct {
	Steps    []SetupStep `json:"steps"`
	AllAuto  bool        `json:"allAuto"`  // true if every step ran automatically
	NeedHelp bool        `json:"needHelp"` // true if extension still needs loading
}

// SetupStep is one step in the setup wizard.
type SetupStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok | skipped | failed | manual
	Detail  string `json:"detail,omitempty"`
}

// Setup runs the full first-time setup wizard:
//
//  1. build awc + awc-host binaries (if Go available)
//  2. check extension key + ID consistency
//  3. install native-messaging host
//  4. add bin/ to PATH (shell rc)
//  5. detect whether the extension is loaded; if not, open chrome://extensions
//     and print precise load instructions
//
// The only step that can never be fully automated on a stock Chrome is
// loading the unpacked extension — Chrome disallows silent installs of
// unpacked extensions by design.
func Setup() (*SetupResult, error) {
	res := &SetupResult{}
	root, err := projectRoot()
	if err != nil {
		return nil, err
	}

	// Step 1: verify prebuilt binaries exist.
	res.Steps = append(res.Steps, verifyBinaries(root))

	// Step 2: key consistency.
	res.Steps = append(res.Steps, checkKeyConsistency(root))

	// Step 3: install host.
	if installStep := doInstall(root); installStep.Status != "" {
		res.Steps = append(res.Steps, installStep)
	}

	// Step 4: PATH.
	res.Steps = append(res.Steps, ensurePATH(root))

	// Step 5: install skills to ~/.agents/skills/ so any AI agent (ZCode,
	// Claude Code, Cursor) can configure login and build business CLIs.
	res.Steps = append(res.Steps, installSkills(root))

	// Step 6: extension load detection.
	res.Steps = append(res.Steps, ensureExtensionLoaded(root))

	for _, s := range res.Steps {
		if s.Status == "manual" || s.Status == "failed" {
			res.NeedHelp = true
		}
	}
	res.AllAuto = !res.NeedHelp
	return res, nil
}

// verifyBinaries checks that the prebuilt binaries exist in the install root.
// Unlike the old buildBinaries, it never compiles — the distributed package
// must ship the binaries. In a dev checkout, the user runs `go build` once
// manually; after that, the binaries are present and this step passes.
func verifyBinaries(root string) SetupStep {
	binDir := filepath.Join(root, "bin")
	missing := []string{}
	for _, name := range []string{"awc", "awc-host"} {
		binName := name
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		if _, err := os.Stat(filepath.Join(binDir, binName)); err != nil {
			missing = append(missing, binName)
		}
	}
	if len(missing) > 0 {
		return SetupStep{Name: "binaries", Status: "failed", Detail: fmt.Sprintf("missing in %s: %s (if this is a dev checkout, run: go build ./... && cp awc awc-host bin/ or just go build -o bin/awc ./cmd/awc && go build -o bin/awc-host ./cmd/host)", binDir, strings.Join(missing, ", "))}
	}
	return SetupStep{Name: "binaries", Status: "ok", Detail: "awc + awc-host present in bin/"}
}

// checkKeyConsistency verifies the extension manifest has a real key (not the
// placeholder) and that the ExtensionID constant is non-empty.
func checkKeyConsistency(root string) SetupStep {
	manifestPath := filepath.Join(root, "extension", "manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return SetupStep{Name: "key", Status: "failed", Detail: "cannot read extension/manifest.json: " + err.Error()}
	}
	body := string(b)
	if strings.Contains(body, "REPLACE_WITH") || !strings.Contains(body, `"key"`) {
		return SetupStep{Name: "key", Status: "failed", Detail: "manifest.json still has a placeholder key; run: openssl genrsa -out .keys/extension.pem 2048 and regenerate"}
	}
	if ExtensionID == "" || strings.Contains(ExtensionID, "pending") {
		return SetupStep{Name: "key", Status: "failed", Detail: "ExtensionID constant is not set"}
	}
	return SetupStep{Name: "key", Status: "ok", Detail: "extension key present, ID=" + ExtensionID}
}

// doInstall wraps Install() into a SetupStep.
func doInstall(root string) SetupStep {
	r, err := Install()
	if err != nil {
		return SetupStep{Name: "host", Status: "failed", Detail: err.Error()}
	}
	return SetupStep{Name: "host", Status: "ok", Detail: fmt.Sprintf("manifest at %s, launcher at %s", r.ManifestPath, r.LauncherPath)}
}

// ensurePATH ensures `awc` is on PATH. If awc is already resolvable (npm
// global install, or a prior setup), it reports ok. Otherwise it appends the
// install bin/ dir to the user's shell rc.
func ensurePATH(root string) SetupStep {
	// If awc is already on PATH (e.g. npm global symlink), nothing to do.
	if _, err := exec.LookPath("awc"); err == nil {
		return SetupStep{Name: "path", Status: "ok", Detail: "awc already on PATH"}
	}

	binDir := filepath.Join(root, "bin")
	exportLine := fmt.Sprintf(`export PATH="%s:$PATH"`, binDir)

	rc := shellRCPath()
	if rc == "" {
		return SetupStep{Name: "path", Status: "skipped", Detail: "unknown shell; add '" + exportLine + "' to your shell rc manually"}
	}
	content, _ := os.ReadFile(rc)
	if strings.Contains(string(content), binDir) {
		return SetupStep{Name: "path", Status: "ok", Detail: "already in " + rc}
	}
	f, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return SetupStep{Name: "path", Status: "skipped", Detail: "cannot write " + rc + "; add '" + exportLine + "' manually"}
	}
	defer f.Close()
	fmt.Fprintln(f, "\n# added by awc sys:setup")
	fmt.Fprintln(f, exportLine)
	return SetupStep{Name: "path", Status: "ok", Detail: "added to " + rc + " (run: source " + rc + ")"}
}

// shellRCPath returns the rc file for the current shell.
func shellRCPath() string {
	shell := os.Getenv("SHELL")
	home, _ := os.UserHomeDir()
	switch {
	case strings.Contains(shell, "zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.Contains(shell, "bash"):
		return filepath.Join(home, ".bashrc")
	case strings.Contains(shell, "fish"):
		return filepath.Join(home, ".config", "fish", "config.fish")
	}
	// Fallback: guess .zshrc on macOS, .bashrc otherwise.
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, ".zshrc")
	}
	return filepath.Join(home, ".bashrc")
}

// ensureExtensionLoaded probes whether the extension is connected by asking
// the host for a live tabs list. Unpacked extensions do not appear in Chrome's
// Preferences file, so an active probe is the only reliable local signal.
func ensureExtensionLoaded(root string) SetupStep {
	extDir := filepath.Join(root, "extension")

	if isExtensionConnected() {
		return SetupStep{Name: "extension", Status: "ok", Detail: "extension connected (tabs:list responded)"}
	}

	// Not loaded: open chrome://extensions to make the manual step as easy
	// as possible. This is the one thing Chrome will not let us automate.
	openExtensionsPage()
	return SetupStep{
		Name:   "extension",
		Status: "manual",
		Detail: fmt.Sprintf("load unpacked extension from:\n    %s\n  chrome://extensions is now open. Enable Developer mode, click 'Load unpacked', select the folder above. Expected ID: %s", extDir, ExtensionID),
	}
}

// isExtensionConnected does a live probe: it asks the host for tabs.query,
// which only succeeds if the extension is loaded and its native port is up.
func isExtensionConnected() bool {
	return probeHostViaSocket("tabs.query")
}

// openExtensionsPage opens chrome://extensions in the default browser.
func openExtensionsPage() {
	url := "chrome://extensions/"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	_ = cmd.Start()
}

// installSkills copies bundled skills to the user-level skill directories of
// AI agents. Different agents look in different places:
//
//   ZCode:     ~/.agents/skills/  and  ~/.zcode/skills/
//   Claude Code: ~/.claude/skills/
//   Cursor:    ~/.cursor/skills/
//   Codex:     ~/.codex/skills/
//
// The SKILL.md format is identical across all of them, so the same file is
// copied to whichever directories the user selects (or all of them).
//
// If stdin is a terminal, the user is prompted to choose which agents to
// install for. If non-interactive (piped/CI), all known targets are used.
func installSkills(root string) SetupStep {
	srcDir := filepath.Join(root, ".agents", "skills")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return SetupStep{Name: "skills", Status: "skipped", Detail: "no bundled skills found"}
	}
	var skillNames []string
	for _, e := range entries {
		if e.IsDir() {
			skillNames = append(skillNames, e.Name())
		}
	}
	if len(skillNames) == 0 {
		return SetupStep{Name: "skills", Status: "skipped", Detail: "no skills found in bundle"}
	}

	// Known agent targets: display name → directory under home.
	targets := []struct {
		Label string
		Dir   string // relative to home
	}{
		{"ZCode", ".agents/skills"},
		{"Claude Code", ".claude/skills"},
		{"Cursor", ".cursor/skills"},
		{"Codex", ".codex/skills"},
	}

	home, _ := os.UserHomeDir()

	// Decide which targets: prompt if interactive, else install all.
	var selected []int
	if isInteractive() {
		selected = promptAgentTargets(targets)
	} else {
		for i := range targets {
			selected = append(selected, i)
		}
	}
	if len(selected) == 0 {
		return SetupStep{Name: "skills", Status: "skipped", Detail: "no agent selected"}
	}

	// Detect which agent dirs already exist to give better feedback.
	var installed []string
	var failed []string
	for _, idx := range selected {
		t := targets[idx]
		dstDir := filepath.Join(home, filepath.FromSlash(t.Dir))
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			failed = append(failed, t.Label)
			continue
		}
		for _, name := range skillNames {
			src := filepath.Join(srcDir, name)
			dst := filepath.Join(dstDir, name)
			if err := copyDir(src, dst); err != nil {
				failed = append(failed, t.Label)
				continue
			}
		}
		installed = append(installed, t.Label+" → ~/"+t.Dir)
	}

	if len(failed) > 0 && len(installed) == 0 {
		return SetupStep{Name: "skills", Status: "failed", Detail: "failed for: " + strings.Join(failed, ", ")}
	}
	detail := fmt.Sprintf("%d skill(s) → %s", len(skillNames), strings.Join(installed, ", "))
	if len(failed) > 0 {
		detail += " (failed: " + strings.Join(failed, ", ") + ")"
	}
	return SetupStep{Name: "skills", Status: "ok", Detail: detail}
}

// isInteractive returns true if stdin is connected to a terminal.
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptAgentTargets asks the user which agents to install skills for.
// Returns indices into the targets slice.
func promptAgentTargets(targets []struct {
	Label string
	Dir   string
}) []int {
	fmt.Println()
	fmt.Println("  Which AI agents do you use? (skills will be installed for them)")
	for i, t := range targets {
		exists := ""
		if home, err := os.UserHomeDir(); err == nil {
			if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(t.Dir))); err == nil {
				exists = " (detected)"
			}
		}
		fmt.Printf("    [%d] %s%s\n", i+1, t.Label, exists)
	}
	fmt.Println("    [a] all of the above")
	fmt.Println()
	fmt.Print("  Enter numbers (e.g. 1,3) or 'a' for all [a]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" || input == "a" {
		var all []int
		for i := range targets {
			all = append(all, i)
		}
		return all
	}

	var selected []int
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if n, err := strconv.Atoi(part); err == nil && n >= 1 && n <= len(targets) {
			selected = append(selected, n-1)
		}
	}
	if len(selected) == 0 {
		// Default to all if input is garbage.
		for i := range targets {
			selected = append(selected, i)
		}
	}
	return selected
}

// copyDir recursively copies src to dst, preserving file permissions.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
