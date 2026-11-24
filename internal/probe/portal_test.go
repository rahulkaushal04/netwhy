package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func runPortal(url string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return Portal{URL: url}.Run(ctx)
}

func TestPortal(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    Status
	}{
		{"open network", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, Pass},
		{"portal redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://login.example/", http.StatusFound)
		}, Fail},
		{"portal login page", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<html>login</html>")) }, Fail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			if got := runPortal(srv.URL); got.Status != tc.want {
				t.Errorf("status = %s (%s), want %s", got.Status, got.Summary, tc.want)
			}
		})
	}
}

func TestPortalRedirectIsShown(t *testing.T) {
	srv := httptest.NewServer(http.RedirectHandler("http://login.example/", http.StatusFound))
	defer srv.Close()

	if got := runPortal(srv.URL); !strings.Contains(got.Evidence.Received, "login.example") {
		t.Errorf("evidence %q should name the redirect target", got.Evidence.Received)
	}
}

func TestPortalUnreachableIsInconclusive(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	if got := runPortal(url); got.Status != Warn {
		t.Errorf("status = %s, want warn when the check itself cannot run", got.Status)
	}
}
