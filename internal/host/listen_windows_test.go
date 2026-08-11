//go:build windows

package host

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func TestListenWindowsCurrentUserACL(t *testing.T) {
	path := fmt.Sprintf(`\\.\pipe\awc-host-test-%d-%d`, os.Getpid(), time.Now().UnixNano())
	ln, err := listen(context.Background(), path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
		accepted <- err
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, path)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()
	if err := <-accepted; err != nil {
		t.Fatalf("accept: %v", err)
	}
}
