//go:build !windows

package host

import (
	"context"
	"net"
	"os"
	"path/filepath"

	"github.com/agent/web-cli/internal/ipc"
)

// listen opens the Unix domain socket used by CLI clients.
func listen(ctx context.Context, path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// Remove a stale socket file from a previous, unclean shutdown.
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// ensure ipc.Endpoint is referenced on non-windows so the import is always
// live even if host.go's usage is compile-time optimised.
var _ = ipc.Endpoint
