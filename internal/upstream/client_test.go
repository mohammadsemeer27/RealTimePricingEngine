package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/metrics"
)

func TestClientConnectAndReceive(t *testing.T) {
	mockServer := NewMockServer(WithMockTickInterval(20 * time.Millisecond))
	defer mockServer.Close()

	m := metrics.NewMetrics()
	var receivedTicks atomic.Uint64
	var wg sync.WaitGroup
	wg.Add(3)

	client := NewClient(ClientConfig{
		URL:              mockServer.URL(),
		APIKey:           "test_key",
		SecretKey:        "test_secret",
		Symbols:          []string{"EURUSD", "BTCUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		ReconnectMax:     500 * time.Millisecond,
		ReadTimeout:      2 * time.Second,
		Metrics:          m,
		Handler: func(ctx context.Context, raw market.RawTick) {
			if receivedTicks.Add(1) <= 3 {
				wg.Done()
			}
		},
	})

	client.Start()
	defer client.Close()

	// Wait for at least 3 ticks to be received
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success!
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for ticks, received %d", receivedTicks.Load())
	}

	if receivedTicks.Load() < 3 {
		t.Errorf("expected at least 3 ticks, got %d", receivedTicks.Load())
	}
}

func TestClientAuthFailureHandling(t *testing.T) {
	// Configure mock server to reject auth
	mockServer := NewMockServer(WithMockAuthFailure(true))
	defer mockServer.Close()

	m := metrics.NewMetrics()
	client := NewClient(ClientConfig{
		URL:              mockServer.URL(),
		APIKey:           "bad_key",
		SecretKey:        "bad_secret",
		Symbols:          []string{"EURUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		Metrics:          m,
	})

	client.Start()
	defer client.Close()

	// Give client time to attempt auth
	time.Sleep(200 * time.Millisecond)

	snap := m.GetSnapshot()
	if snap.AuthFailures == 0 {
		t.Errorf("expected at least 1 auth failure recorded in metrics, got %d", snap.AuthFailures)
	}
	if snap.UpstreamReconnects > 0 {
		t.Errorf("expected 0 reconnect attempts for auth failure, got %d", snap.UpstreamReconnects)
	}
}

func TestValidateAuthResponse_Success(t *testing.T) {
	validPayloads := []string{
		`{"status":"success"}`,
		`{"status":"ok"}`,
		`{"status":"authenticated"}`,
		`{"status":"connected"}`,
		`{"event":"authenticated"}`,
		`{"event":"success"}`,
		`{"action":"auth","status":"success"}`,
		`{"action":"auth","status":"ok"}`,
		`{"authenticated":true}`,
		`{"success":true}`,
	}

	for _, payload := range validPayloads {
		if err := validateAuthResponse([]byte(payload)); err != nil {
			t.Errorf("expected payload %s to be accepted as success, got error: %v", payload, err)
		}
	}
}

func TestValidateAuthResponse_Rejected(t *testing.T) {
	rejectedPayloads := []struct {
		name    string
		payload string
	}{
		{"LiveFXHub exact error event", `{"event":"error","message":"invalid credentials"}`},
		{"Status error with message", `{"status":"error","message":"invalid credentials"}`},
		{"Status error with error field", `{"status":"error","error":"unauthorized"}`},
		{"Status failed", `{"status":"failed"}`},
		{"Status unauthorized", `{"status":"unauthorized"}`},
		{"Status rejected", `{"status":"rejected"}`},
		{"Direct error string", `{"error":"invalid signature"}`},
		{"Status code 401", `{"code":401,"message":"unauthorized"}`},
		{"Status code 4003", `{"code":4003,"message":"invalid credentials"}`},
		{"Authenticated flag false", `{"authenticated":false}`},
		{"Success flag false", `{"success":false}`},
	}

	for _, tc := range rejectedPayloads {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAuthResponse([]byte(tc.payload))
			if err == nil {
				t.Fatalf("expected payload %s to be rejected, got nil", tc.payload)
			}
			if !errors.Is(err, ErrAuthFailed) {
				t.Errorf("expected ErrAuthFailed, got %v", err)
			}
		})
	}
}

func TestValidateAuthResponse_MalformedAndIncomplete(t *testing.T) {
	malformedPayloads := []struct {
		name    string
		payload string
	}{
		{"Invalid JSON syntax", `not valid json`},
		{"Empty JSON object", `{}`},
		{"Unrelated fields", `{"foo":"bar"}`},
		{"Action without status", `{"action":"auth"}`},
		{"Random numbers", `{"count":123}`},
	}

	for _, tc := range malformedPayloads {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAuthResponse([]byte(tc.payload))
			if err == nil {
				t.Fatalf("expected payload %s to be rejected, got nil", tc.payload)
			}
			if !errors.Is(err, ErrAuthFailed) {
				t.Errorf("expected ErrAuthFailed, got %v", err)
			}
		})
	}
}

func TestClient_CloseCode4003_DuringAuth(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var connections atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read auth request
		_, _, _ = conn.ReadMessage()

		// Send WebSocket close frame with code 4003: invalid credentials
		closeMsg := websocket.FormatCloseMessage(4003, "invalid credentials")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	m := metrics.NewMetrics()

	client := NewClient(ClientConfig{
		URL:              wsURL,
		APIKey:           "test_key",
		SecretKey:        "test_secret",
		Symbols:          []string{"EURUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		Metrics:          m,
	})

	client.Start()
	defer client.Close()

	time.Sleep(300 * time.Millisecond)

	snap := m.GetSnapshot()
	if snap.AuthFailures == 0 {
		t.Errorf("expected at least 1 auth failure recorded, got %d", snap.AuthFailures)
	}
	if snap.UpstreamReconnects > 0 {
		t.Errorf("expected 0 reconnects on close code 4003, got %d", snap.UpstreamReconnects)
	}
	// Connection count should be at most 1 (no continuous reconnection)
	if count := connections.Load(); count > 1 {
		t.Errorf("expected client to halt after 1 attempt on code 4003, made %d connections", count)
	}
}

func TestClient_CloseCode4003_DuringStream(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var connections atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// 1. Read auth
		_, _, _ = conn.ReadMessage()

		// 2. Reply with success
		_ = conn.WriteJSON(map[string]interface{}{
			"action": "auth",
			"status": "success",
		})

		// 3. Read subscribe
		_, _, _ = conn.ReadMessage()

		// 4. Close with code 4003
		closeMsg := websocket.FormatCloseMessage(4003, "invalid credentials")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	m := metrics.NewMetrics()

	client := NewClient(ClientConfig{
		URL:              wsURL,
		APIKey:           "test_key",
		SecretKey:        "test_secret",
		Symbols:          []string{"EURUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		Metrics:          m,
	})

	client.Start()
	defer client.Close()

	time.Sleep(300 * time.Millisecond)

	snap := m.GetSnapshot()
	if snap.AuthFailures == 0 {
		t.Errorf("expected at least 1 auth failure recorded on code 4003 during stream, got %d", snap.AuthFailures)
	}
	if count := connections.Load(); count > 1 {
		t.Errorf("expected client to halt reconnect loop on code 4003 during stream, made %d connections", count)
	}
}

func TestClient_SubscriptionGatedOnAuth(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var receivedSubscribe atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read auth
		_, _, _ = conn.ReadMessage()

		// Send rejection response (LiveFXHub style)
		_ = conn.WriteJSON(map[string]interface{}{
			"event":   "error",
			"message": "invalid credentials",
		})

		// Wait to see if client sends subscribe
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, msg, err := conn.ReadMessage()
		if err == nil {
			var m map[string]interface{}
			if err := json.Unmarshal(msg, &m); err == nil && m["action"] == "subscribe" {
				receivedSubscribe.Store(true)
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	m := metrics.NewMetrics()

	client := NewClient(ClientConfig{
		URL:              wsURL,
		APIKey:           "bad_key",
		SecretKey:        "bad_secret",
		Symbols:          []string{"EURUSD"},
		ReconnectInitial: 50 * time.Millisecond,
		Metrics:          m,
	})

	client.Start()
	defer client.Close()

	time.Sleep(300 * time.Millisecond)

	if receivedSubscribe.Load() {
		t.Errorf("subscription must NOT be sent after rejected authentication")
	}
}

func TestClient_RetryPolicyConfigurable(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	var connAttempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connAttempts.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, _, _ = conn.ReadMessage()
		_ = conn.WriteJSON(map[string]interface{}{
			"event":   "error",
			"message": "invalid credentials",
		})
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	m := metrics.NewMetrics()

	// Configure client with MaxAuthRetries: 2
	client := NewClient(ClientConfig{
		URL:              wsURL,
		APIKey:           "bad_key",
		SecretKey:        "bad_secret",
		Symbols:          []string{"EURUSD"},
		ReconnectInitial: 20 * time.Millisecond,
		MaxAuthRetries:   2,
		Metrics:          m,
	})

	client.Start()
	defer client.Close()

	time.Sleep(300 * time.Millisecond)

	// Initial attempt (1) + 2 retries = 3 attempts total
	attempts := connAttempts.Load()
	if attempts != 3 {
		t.Errorf("expected exactly 3 attempts (1 initial + 2 retries), got %d", attempts)
	}
}
