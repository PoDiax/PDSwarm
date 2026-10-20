package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyMiddleware(t *testing.T) {
	trusted := []string{"127.0.0.1/32", "10.0.0.0/8"}
	mw := TrustedProxyMiddleware(trusted)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}))

	// Case 1: Untrusted remote IP with spoofed headers -> should NOT change RemoteAddr
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "198.51.100.1:12345"
	req1.Header.Set("X-Forwarded-For", "203.0.113.195")
	req1.Header.Set("CF-Connecting-IP", "203.0.113.195")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Body.String() != "198.51.100.1:12345" {
		t.Errorf("expected 198.51.100.1:12345, got %s", rec1.Body.String())
	}

	// Case 2: Trusted remote IP with CF-Connecting-IP -> should use CF-Connecting-IP
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "127.0.0.1:54321"
	req2.Header.Set("CF-Connecting-IP", "203.0.113.50")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Body.String() != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", rec2.Body.String())
	}

	// Case 3: Trusted remote IP with X-Real-IP -> should use X-Real-IP
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "10.0.0.2:54321"
	req3.Header.Set("X-Real-IP", "203.0.113.51")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Body.String() != "203.0.113.51" {
		t.Errorf("expected 203.0.113.51, got %s", rec3.Body.String())
	}

	// Case 4: Trusted remote IP with X-Forwarded-For chain -> should use the first valid IP
	req4 := httptest.NewRequest("GET", "/", nil)
	req4.RemoteAddr = "127.0.0.1:54321"
	req4.Header.Set("X-Forwarded-For", "203.0.113.52, 10.0.0.1")
	rec4 := httptest.NewRecorder()
	handler.ServeHTTP(rec4, req4)
	if rec4.Body.String() != "203.0.113.52" {
		t.Errorf("expected 203.0.113.52, got %s", rec4.Body.String())
	}
}
