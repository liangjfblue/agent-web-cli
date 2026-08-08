// host is the native-messaging entry point launched by Chrome.
//
// Build it as a standalone binary (e.g. awc-host) and register it via the
// native-messaging manifest. On run, it connects stdio to the extension and
// serves CLI connections on the local socket.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/agent/web-cli/internal/host"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := host.Run(ctx); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
