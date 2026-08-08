//go:build windows

package host

import (
	"context"
	"net"
)

// listen opens the Windows named pipe used by CLI clients.
// A full npipe implementation requires github.com/Microsoft/go-winio; this
// stub keeps the windows build green and is completed when the windows
// transport is wired up.
func listen(ctx context.Context, path string) (net.Listener, error) {
	return nil, net.ErrClosed
}
