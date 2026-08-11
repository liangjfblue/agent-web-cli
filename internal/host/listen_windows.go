//go:build windows

package host

import (
	"context"
	"fmt"
	"net"
	"os/user"

	"github.com/Microsoft/go-winio"
)

// listen opens the Windows named pipe used by CLI clients.
func listen(ctx context.Context, path string) (net.Listener, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("resolve current user SID: %w", err)
	}
	if u.Uid == "" {
		return nil, fmt.Errorf("resolve current user SID: empty SID")
	}
	config := &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + u.Uid + ")",
		MessageMode:        false,
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	}
	return winio.ListenPipe(path, config)
}
