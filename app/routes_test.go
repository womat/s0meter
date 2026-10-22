package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/womat/golib/v2/web"
	"github.com/womat/mqtt"
	"github.com/womat/s0meter/app/service/health"
	"github.com/womat/s0meter/app/service/s0meters"
)

// newTestApp returns an App with routes set up, no meters and no MQTT broker. Each configure
// function changes the config before the routes are built.
func newTestApp(t *testing.T, configure ...func(*Config)) *App {
	t.Helper()
	cfg := NewConfig()
	cfg.Webserver.Tokens = []web.Token{{Name: "test", Hash: web.HashToken("test-key")}}
	for _, f := range configure {
		f(cfg)
	}
	app := &App{config: cfg, web: &http.Server{}, meters: s0meters.New()}
	app.SetupRoutes()
	return app
}

// serve sends a GET for path, with token as bearer token unless it is empty.
func serve(app *App, path, token string) *httptest.ResponseRecorder {
	return serveMethod(app, http.MethodGet, path, token)
}

// serveMethod sends a request with method for path from 127.0.0.1, with token as bearer token
// unless it is empty.
func serveMethod(app *App, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "127.0.0.1:1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	app.web.Handler.ServeHTTP(rec, req)
	return rec
}

func TestUIIsPublicAndOnlyAtRoot(t *testing.T) {
	app := newTestApp(t)

	rec := serve(app, "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200 without a token", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	// The page may only talk back to this server; everything else stays off.
	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'none'", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("Content-Security-Policy %q lacks %q", csp, directive)
		}
	}

	// "OPTIONS /" matches every path, so an unknown GET is a 405 rather than a 404.
	if rec := serve(app, "/unknown", ""); rec.Code == http.StatusOK {
		t.Error("GET /unknown = 200: the page must not catch every path")
	}
}

func TestHealthReportsMeterStatusAndMqtt(t *testing.T) {
	app := newTestApp(t)

	if rec := serve(app, "/health", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /health without a token = %d, want 401", rec.Code)
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
	if _, ok := got["mqttBroker"]; ok {
		t.Errorf("mqttBroker = %v without a broker, want it omitted", got["mqttBroker"])
	}
}

// The web page shows the broker in the tooltip of the MQTT pill; user and password from the
// connection URL must never reach it.
func TestHealthReportsBrokerWithoutCredentials(t *testing.T) {
	app := newTestApp(t)
	app.config.MQTT = &MQTTConfig{Connection: "tcp://user:secret@192.168.1.5:1883"}

	rec := serve(app, "/health", "test-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "secret") || strings.Contains(body, "user:") || strings.Contains(body, "user@") {
		t.Errorf("/health leaks the broker credentials: %s", body)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["mqttBroker"] != "192.168.1.5:1883" {
		t.Errorf("mqttBroker = %v, want host:port 192.168.1.5:1883", got["mqttBroker"])
	}
}

func TestBrokerHost(t *testing.T) {
	for in, want := range map[string]string{
		"tcp://192.168.1.5:1883":              "192.168.1.5:1883",
		"tcp://user:secret@mqtt.example:1883": "mqtt.example:1883",
		"ssl://user@[::1]:8883":               "[::1]:8883",
		"tcp://%zz":                           "",
		"":                                    "",
	} {
		if got := brokerHost(in); got != want {
			t.Errorf("brokerHost(%q) = %q, want %q", in, got, want)
		}
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
}

// The Pi may be offline, so the page loads nothing from elsewhere. The SVG namespace is an
// identifier, not a request.
func TestUILoadsNoExternalResources(t *testing.T) {
	page := strings.ReplaceAll(string(uiPage), "http://www.w3.org/2000/svg", "")
	if urls := regexp.MustCompile(`(?i)(https?:)?//[a-z0-9.-]+\.[a-z]{2,}`).FindAllString(page, -1); len(urls) > 0 {
		t.Errorf("page references external resources: %v", urls)
	}
}

func TestVersionIsPublic(t *testing.T) {
	rec := serve(newTestApp(t), "/version", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /version = %d, want 200 without a token", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["app"] != MODULE || got["appVersion"] != VERSION {
		t.Errorf("/version = %v", got)
	}
}

// The IP filter wraps everything, so a blocked client gets neither the public page nor a
// protected route, even with a valid token.
func TestIPFilterGuardsEveryRoute(t *testing.T) {
	for name, tt := range map[string]struct {
		configure func(*Config)
		want      int
	}{
		"blocked":         {func(c *Config) { c.Webserver.BlockedIPs = []string{"127.0.0.1"} }, http.StatusForbidden},
		"blocked network": {func(c *Config) { c.Webserver.BlockedIPs = []string{"127.0.0.0/8"} }, http.StatusForbidden},
		"not allowed":     {func(c *Config) { c.Webserver.AllowedIPs = []string{"192.168.0.0/16"} }, http.StatusForbidden},
		"allowed":         {func(c *Config) { c.Webserver.AllowedIPs = []string{"127.0.0.1"} }, http.StatusOK},
		"no filter":       {func(*Config) {}, http.StatusOK},
	} {
		t.Run(name, func(t *testing.T) {
			app := newTestApp(t, tt.configure)
			for _, path := range []string{"/", "/health"} {
				if rec := serve(app, path, "test-key"); rec.Code != tt.want {
					t.Errorf("GET %s = %d, want %d", path, rec.Code, tt.want)
				}
			}
		})
	}
}

// The API is read-only: a preflight needs no token and advertises GET and OPTIONS only, and
// any other method is refused.
func TestCORSAndMethods(t *testing.T) {
	app := newTestApp(t)

	rec := serveMethod(app, http.MethodOptions, "/meters", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS /meters = %d, want 204 without a token", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q, want GET, OPTIONS", got)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := serveMethod(app, method, "/meters", "test-key"); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /meters = %d, want 405", method, rec.Code)
		}
	}
}

// A broker that is configured but not connected shows as disconnected, not as disabled.
func TestHealthReportsDisconnectedBroker(t *testing.T) {
	app := newTestApp(t)
	app.config.MQTT = &MQTTConfig{Connection: "tcp://192.168.1.5:1883"}
	app.mqtt = &mqtt.Handler{} // no client: IsConnectionOpen is false

	rec := serve(app, "/health", "test-key")
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["mqtt"] != health.MqttDisconnected {
		t.Errorf("mqtt = %v, want %q", got["mqtt"], health.MqttDisconnected)
	}
}
