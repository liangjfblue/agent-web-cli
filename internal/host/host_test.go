package host

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"sync"
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

type yieldingWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *yieldingWriter) Write(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *yieldingWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buf.Bytes()...)
}

func TestHostSerializesNativeWrites(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &yieldingWriter{}
	h := &Host{
		extensionOut:    w,
		extensionWrites: make(chan extensionWrite),
		extensionDone:   make(chan struct{}),
		writerDone:      make(chan struct{}),
	}
	go h.writeExtensionLoop(ctx)

	const count = 40
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- h.writeExtension(ChromeRequest{Tid: string(rune('a' + i)), Op: "tabs.query"})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("writeExtension: %v", err)
		}
	}

	data := w.Bytes()
	seen := 0
	for len(data) > 0 {
		if len(data) < 4 {
			t.Fatalf("truncated native frame header: %d bytes", len(data))
		}
		n := int(binary.LittleEndian.Uint32(data[:4]))
		data = data[4:]
		if n > len(data) {
			t.Fatalf("interleaved native frame: length %d exceeds remaining %d", n, len(data))
		}
		var req ChromeRequest
		if err := json.Unmarshal(data[:n], &req); err != nil {
			t.Fatalf("invalid native frame JSON: %v", err)
		}
		data = data[n:]
		seen++
	}
	if seen != count {
		t.Fatalf("decoded %d frames, want %d", seen, count)
	}
}
