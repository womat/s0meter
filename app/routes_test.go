package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/womat/golib/web"
	"github.com/womat/s0meter/app/service/health"
	"github.com/womat/s0meter/app/service/s0meters"
)

// newTestApp returns an App with routes set up, no meters and no MQTT broker.
func newTestApp(t *testing.T) *App {
	t.Helper()
	cfg := NewConfig()
	cfg.Webserver.ApiKey = "test-key"
	app := &App{config: cfg, web: &http.Server{}, meters: s0meters.New()}
	app.SetupRoutes()
	return app
}

func serve(app *App, path, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "127.0.0.1:1234"
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	rec := httptest.NewRecorder()
	app.web.Handler.ServeHTTP(rec, req)
	return rec
}

func TestUIIsPublicAndOnlyAtRoot(t *testing.T) {
	app := newTestApp(t)

	rec := serve(app, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200 without API key", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("page is served without a Content-Security-Policy")
	}

	// "OPTIONS /" matches every path, so an unknown GET is a 405 rather than a 404.
	if rec := serve(app, "/unknown", ""); rec.Code == http.StatusOK {
		t.Error("GET /unknown = 200: the page must not catch every path")
	}
}

func TestHealthReportsMeterStatusAndMqtt(t *testing.T) {
	app := newTestApp(t)

	if rec := serve(app, "/health", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /health without key = %d, want 401", rec.Code)
	}

	rec := serve(app, "/health", "test-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["mqtt"] != health.MqttDisabled {
		t.Errorf("mqtt = %v, want %q without a broker", got["mqtt"], health.MqttDisabled)
	}
	if _, ok := got["meters"].(map[string]any); !ok {
		t.Errorf("meters = %v, want an object", got["meters"])
	}
	if _, ok := got["droppedEvents"]; ok {
		t.Error("droppedEvents is still reported at the top level; it moved into meters")
	}
}

func TestErrorsAreJSON(t *testing.T) {
	app := newTestApp(t)

	for _, tc := range []struct {
		path, key string
		code      int
		msg       string
	}{
		{"/meters", "", http.StatusUnauthorized, "not authorized"},
		{"/meters/nope", "test-key", http.StatusNotFound, "meter nope not found"},
	} {
		rec := serve(app, tc.path, tc.key)
		if rec.Code != tc.code {
			t.Errorf("GET %s = %d, want %d", tc.path, rec.Code, tc.code)
		}
		var got web.ApiError
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Error != tc.msg {
			t.Errorf("GET %s body = %s, want {\"error\":%q}", tc.path, rec.Body, tc.msg)
		}
	}

	if rec := serve(app, "/ready", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /ready without a broker = %d, want 200", rec.Code)
	}
}
