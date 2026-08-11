package host

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestRunReturnsWhenNativeInputCloses(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- run(context.Background(), bytes.NewReader(nil), io.Discard)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("host did not exit after native messaging input closed")
	}
}
