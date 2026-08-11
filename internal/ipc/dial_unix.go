//go:build !windows

package ipc

import (
	"context"
	"net"
)

func dial(ctx context.Context, endpoint string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", endpoint)
}
