package websocket

import (
	"encoding/json"

	"github.com/mohammadsemeer27/RealTimePricingEngine/internal/market"
)

// ClientAction defines actions sent by downstream clients.
type ClientAction string

const (
	ActionSubscribe   ClientAction = "subscribe"
	ActionUnsubscribe ClientAction = "unsubscribe"
	ActionSnapshot    ClientAction = "snapshot"
	ActionPing        ClientAction = "ping"
)

// ClientMessage represents an incoming message from a downstream WebSocket client.
type ClientMessage struct {
	Action  ClientAction `json:"action"`
	Symbols []string     `json:"symbols,omitempty"`
}

// ServerResponse represents an acknowledgment or error response.
type ServerResponse struct {
	Type    string   `json:"type"` // "ack", "error", "pong"
	Action  string   `json:"action,omitempty"`
	Status  string   `json:"status,omitempty"`
	Message string   `json:"message,omitempty"`
	Symbols []string `json:"symbols,omitempty"`
}

// Envelope is a helper to encode server payloads.
func EncodeAck(action string, symbols []string) ([]byte, error) {
	return json.Marshal(ServerResponse{
		Type:    "ack",
		Action:  action,
		Status:  "ok",
		Symbols: symbols,
	})
}

// EncodeError encodes an error response.
func EncodeError(msg string) ([]byte, error) {
	return json.Marshal(ServerResponse{
		Type:    "error",
		Message: msg,
	})
}

// EncodeSnapshot encodes a full initial market snapshot.
func EncodeSnapshot(snapshot market.MarketSnapshot) ([]byte, error) {
	type SnapshotMsg struct {
		Type string                `json:"type"`
		Data market.MarketSnapshot `json:"data"`
	}
	return json.Marshal(SnapshotMsg{
		Type: "snapshot",
		Data: snapshot,
	})
}

// EncodeDelta encodes an optimized compact delta update.
func EncodeDelta(delta market.MarketDelta) ([]byte, error) {
	return json.Marshal(delta)
}
