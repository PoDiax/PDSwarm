package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWSLogWriter_SanitizesInvalidUTF8(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		writer := &wsLogWriter{
			conn: conn,
			ctx:  ctx,
		}

		// Write invalid UTF-8 bytes
		corrupt := []byte("hello \xff\xfe world \x80\x81")
		n, err := writer.Write(corrupt)
		if err != nil {
			t.Errorf("writer.Write failed: %v", err)
		}
		if n != len(corrupt) {
			t.Errorf("expected written %d, got %d", len(corrupt), n)
		}

		// Write valid UTF-8
		valid := []byte("clean line\n")
		n, err = writer.Write(valid)
		if err != nil {
			t.Errorf("writer.Write failed: %v", err)
		}
		if n != len(valid) {
			t.Errorf("expected written %d, got %d", len(valid), n)
		}
	}))
	defer s.Close()

	wsURL := "ws" + strings.TrimPrefix(s.URL, "http")
	clientConn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer clientConn.CloseNow()

	// Read first frame: should be sanitized UTF-8
	msgType, msg, err := clientConn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read first frame: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected MessageText, got %v", msgType)
	}
	if string(msg) != "hello  world " {
		t.Errorf("expected sanitized 'hello  world ', got %q", string(msg))
	}

	// Read second frame
	msgType, msg, err = clientConn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read second frame: %v", err)
	}
	if string(msg) != "clean line\n" {
		t.Errorf("expected 'clean line\\n', got %q", string(msg))
	}
}
