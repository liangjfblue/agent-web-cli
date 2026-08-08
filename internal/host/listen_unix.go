//go:build !windows

package host

import (
	"context"
	"net"
	"os"

	"github.com/agent/web-cli/internal/ipc"
)

// listen opens the Unix domain socket used by CLI clients.
func listen(ctx context.Context, path string) (net.Listener, error) {
	// Remove a stale socket file from a previous, unclean shutdown.
	if _, err := os.Stat(path); err == nil {
		os.Remove(path)
	}
	return net.Listen("unix", path)
}

// ensure ipc.Endpoint is referenced on non-windows so the import is always
// live even if host.go's usage is compile-time optimised.
var _ = ipc.Endpoint
