package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// manifestPath returns where Chrome looks for this host's manifest on the
// current platform. Windows relies on a registry entry instead (the file is
// still written next to the launcher there, but registration is via reg).
func manifestPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts", HostName+".json"), nil
	case "linux":
		return filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts", HostName+".json"), nil
	case "windows":
		// On Windows the manifest can live anywhere; the registry default
		// value points to it. We put it in the user's awc data dir so it
		// survives package upgrades.
		return filepath.Join(home, ".awc", "host-manifest", HostName+".json"), nil
	}
	return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
}

// regKey is the Windows registry path under HKCU.
func regKey() string {
	return `HKCU\Software\Google\Chrome\NativeMessagingHosts\` + HostName
}

// registerWindows writes the registry default value pointing to manifestPath.
func registerWindows(manifestPath string) error {
	cmd := exec.Command("reg", "add", regKey(), "/ve", "/t", "REG_SZ", "/d", manifestPath, "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

// BuiltRoot is the project root baked in at compile time via
// -ldflags "-X install.BuiltRoot=/path". Empty in development builds.
var BuiltRoot = ""

// projectRoot finds the agent-web-cli source tree. It tries, in order:
//
//  1. BuiltRoot (compile-time injection, for globally installed binaries)
//  2. the launcher path from an already-registered native-messaging manifest
//     (the launcher points at <root>/.awc/awc-host-launcher or <root>/bin/...)
//  3. the directory of the running executable, walking up to find go.mod
//  4. the current working directory, walking up to find go.mod (dev mode)
func projectRoot() (string, error) {
	// 1. compile-time injection
	if BuiltRoot != "" {
		if isInstallRoot(BuiltRoot) {
			return BuiltRoot, nil
		}
	}

	// 2. reverse-engineer from the registered manifest
	if root := rootFromManifest(); root != "" {
		return root, nil
	}

	// 3. executable location
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		for i := 0; i < 6; i++ {
			if isInstallRoot(dir) {
				return dir, nil
			}
			if dir == filepath.Dir(dir) {
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	// 4. cwd (development fallback)
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isInstallRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("awc install: could not locate project root (run from the source tree, or reinstall with: awc sys:install)")
}

// isInstallRoot checks whether dir is a self-contained awc install: it must
// contain an extension/ directory AND a bin/awc-host binary. This works for
// both source trees (dev) and npm packages (distribution) — neither requires
// go.mod to exist at runtime.
func isInstallRoot(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "extension")); err != nil {
		return false
	}
	binName := "awc-host"
	if runtime.GOOS == "windows" {
		binName = "awc-host.exe"
	}
	if _, err := os.Stat(filepath.Join(dir, "bin", binName)); err != nil {
		return false
	}
	return true
}

// rootFromManifest reads the registered native-messaging manifest and derives
// the project root from the launcher path. The launcher lives at either:
//   <root>/.awc/awc-host-launcher       (unix)
//   <root>/bin/awc-host.exe             (windows)
func rootFromManifest() string {
	path, err := manifestPath()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	launcher := m.Path
	if launcher == "" {
		return ""
	}
	// launcher is at <root>/.awc/launcher or <root>/bin/host[.exe]
	// parent of the launcher's dir is the root in the .awc case;
	// the launcher's dir itself is the root in the bin/ case.
	dir := filepath.Dir(launcher) // .awc or bin
	candidate := filepath.Dir(dir)
	if isInstallRoot(candidate) {
		return candidate
	}
	if isInstallRoot(dir) {
		return dir
	}
	return ""
}
