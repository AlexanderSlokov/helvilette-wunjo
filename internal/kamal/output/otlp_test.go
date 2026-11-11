package output

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPPosterSendsJSONAndReturnsTheStatus(t *testing.T) {
	var contentType, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	status, text, err := NewHTTPPoster().Post(server.URL+"/v1/logs", []byte(`{"resourceLogs":[]}`))
	if err != nil || status != 202 || text != "Accepted" || contentType != "application/json" || body != `{"resourceLogs":[]}` {
		t.Fatalf("status %d %q, err %v, content type %q, body %q", status, text, err, contentType, body)
	}
}
