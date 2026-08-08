package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ensureLauncher returns the absolute path to a launcher that Chrome can
// execute to start the host. It expects the prebuilt host binary at
// <installRoot>/bin/awc-host; if absent it errors clearly (no on-the-fly
// compilation — the distributed package must ship the binary).
//
// The launcher itself is written to the user's data dir (~/.awc/launchers/)
// so it survives package upgrades.
func ensureLauncher(installRoot string) (string, error) {
	binName := "awc-host"
	if runtime.GOOS == "windows" {
		binName = "awc-host.exe"
	}
	hostBin := filepath.Join(installRoot, "bin", binName)
	if _, err := os.Stat(hostBin); err != nil {
		return "", fmt.Errorf("host binary not found at %s — the install is incomplete", hostBin)
	}

	launcherDir := launcherDirectory()
	if err := os.MkdirAll(launcherDir, 0o755); err != nil {
		return "", fmt.Errorf("create launcher dir: %w", err)
	}

	if runtime.GOOS == "windows" {
		// On Windows Chrome needs an .exe or .bat launcher. We write a .bat
		// that invokes the host binary.
		launcher := filepath.Join(launcherDir, "awc-host-launcher.bat")
		content := fmt.Sprintf("@echo off\r\n\"%s\" %%*\r\n", strings.ReplaceAll(hostBin, `\`, `\\`))
		if err := os.WriteFile(launcher, []byte(content), 0o755); err != nil {
			return "", err
		}
		return launcher, nil
	}

	// Unix: a small shell script.
	launcher := filepath.Join(launcherDir, "awc-host-launcher")
	content := "#!/bin/sh\nexec " + shellQuote(hostBin) + " \"$@\"\n"
	if err := os.WriteFile(launcher, []byte(content), 0o755); err != nil {
		return "", err
	}
	return launcher, nil
}

// launcherDirectory returns ~/.awc/launchers (created on demand by the
// caller). This is a per-user, per-machine stable location that package
// upgrades never touch.
func launcherDirectory() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".awc", "launchers")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
