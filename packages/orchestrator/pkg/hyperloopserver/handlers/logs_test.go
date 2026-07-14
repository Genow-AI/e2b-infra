//go:build linux

package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIStoreForwardLogsStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "success", status: http.StatusAccepted},
		{name: "client error", status: http.StatusUnprocessableEntity, wantErr: true},
		{name: "server error", status: http.StatusBadGateway, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte("collector diagnostic"))
			}))
			t.Cleanup(server.Close)

			store := &APIStore{collectorClient: *server.Client()}
			err := store.forwardLogs(t.Context(), server.URL, []byte(`{"secret":"not-in-error"}`), 0)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "not-in-error") {
				t.Fatalf("error contains request payload: %v", err)
			}
		})
	}
}
