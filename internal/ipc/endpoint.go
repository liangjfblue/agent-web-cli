// Package ipc handles the local transport between the awc CLI and the
// native host process: a Unix domain socket on macOS/Linux and a Windows
// named pipe on Windows.
//
// The socket path is partitioned per OS user so that two users on the same
// machine never collide.
package ipc

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
)

const (
	// HostName is the Chrome native-messaging host name. It must match the
	// extension's connectNative argument and the installed manifest.
	HostName = "com.awc.host"

	// SockName is the socket filename stem; the OS user id is appended.
	SockName = "awc-host"
)

// Endpoint returns the socket path the host listens on and the CLI dials.
//
// Unix:   $AWC_RUNTIME_DIR/awc-host-<uid>.sock  (default user cache directory)
// Windows: \\.\pipe\awc-host-<uid>
func Endpoint() string {
	return EndpointForProfile("")
}

// EndpointForProfile returns the endpoint for one Chrome profile. An empty
// profile id preserves the legacy endpoint for older extensions and hosts.
func EndpointForProfile(profileID string) string {
	seg := userSegment()
	suffix := ""
	if profileID != "" {
		suffix = "-" + sanitize(profileID)
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\awc-%s%s`, seg, suffix)
	}
	return filepath.Join(RuntimeDir(), fmt.Sprintf("%s-%s%s.sock", SockName, seg, suffix))
}

// RuntimeDir is private to the current user and contains sockets plus the
// small profile registry. It can be overridden for tests and containers.
func RuntimeDir() string {
	if dir := os.Getenv("AWC_RUNTIME_DIR"); dir != "" {
		return dir
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "awc", "run")
	}
	return filepath.Join(os.TempDir(), "awc-"+userSegment())
}

// userSegment returns a filesystem-safe identifier for the current OS user.
func userSegment() string {
	if u, err := user.Current(); err == nil {
		if u.Uid != "" {
			return sanitize(u.Uid)
		}
		if u.Username != "" {
			return sanitize(u.Username)
		}
	}
	return "default"
}

var unsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitize(s string) string {
	return unsafe.ReplaceAllString(s, "_")
}
