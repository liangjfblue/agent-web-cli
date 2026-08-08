// Package install registers the awc native-messaging host with Chrome.
//
// It writes a Chrome native-messaging manifest pointing at a launcher, and
// places it where Chrome expects it:
//
//   - macOS: ~/Library/Application Support/Google/Chrome/NativeMessagingHosts/<name>.json
//   - Linux: ~/.config/google-chrome/NativeMessagingHosts/<name>.json
//   - Windows: registry key HKCU\Software\Google\Chrome\NativeMessagingHosts\<name>
//
// The manifest schema and locations are dictated by Chrome; they are not
// specific to any single project. Everything else here (launcher format,
// host name, extension id handling) is this project's own.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Result describes what was installed.
type Result struct {
	HostName     string `json:"hostName"`
	ManifestPath string `json:"manifestPath"`
	LauncherPath string `json:"launcherPath"`
	ExtensionID  string `json:"extensionId"`
}

// HostName is the Chrome native-messaging host name. It must match the
// extension's connectNative argument.
const HostName = "com.awc.host"

// ExtensionID is the stable id produced by the extension's fixed public key
// in extension/manifest.json. It was computed from the DER-encoded public
// key via SHA-256 → first 16 bytes → mapped 0-9a-f to a-p (Chrome's algorithm).
// To regenerate: see .keys/ and the README "Regenerating the extension key".
const ExtensionID = "klhhipipedegmphifibmojnhbaecjodf"

// Manifest is the Chrome native-messaging manifest structure.
type Manifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// Install registers the host and returns the result. The launcher points at
// the prebuilt host binary; if absent, a shell/bat wrapper is generated.
func Install() (*Result, error) {
	root, err := projectRoot()
	if err != nil {
		return nil, err
	}
	launcher, err := ensureLauncher(root)
	if err != nil {
		return nil, err
	}
	manifest := Manifest{
		Name:           HostName,
		Description:    "Agent Web CLI Native Host",
		Path:           launcher,
		Type:           "stdio",
		AllowedOrigins: []string{fmt.Sprintf("chrome-extension://%s/", ExtensionID)},
	}
	path, err := writeManifest(&manifest)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		if err := registerWindows(path); err != nil {
			return nil, fmt.Errorf("register host (windows): %w", err)
		}
	}
	return &Result{
		HostName:     HostName,
		ManifestPath: path,
		LauncherPath: launcher,
		ExtensionID:  ExtensionID,
	}, nil
}

// Uninstall removes the manifest (and, on Windows, the registry key).
func Uninstall() error {
	if runtime.GOOS == "windows" {
		_ = exec.Command("reg", "delete", regKey(), "/f").Run()
	}
	path, err := manifestPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// writeManifest serialises m to the platform-specific location.
func writeManifest(m *Manifest) (string, error) {
	path, err := manifestPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
