package httpapi

import (
	"testing"

	"github.com/coder/websocket"
)

func TestDecodeWebSocketClientEventRejectsNonTextFrame(t *testing.T) {
	t.Parallel()

	_, err := decodeWebSocketClientEvent(websocket.MessageBinary, []byte(`{"type":"ping"}`))
	if err == nil {
		t.Fatal("decodeWebSocketClientEvent(binary) error = nil")
	}
}

func FuzzDecodeClientEventPayload(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(``),
		[]byte(`null`),
		[]byte(`{`),
		[]byte(`[]`),
		[]byte(`{"type":"ping","request_id":"seed"}`),
		[]byte(`{"type":1}`),
		[]byte(`{"type":"send_message","content":"\u0000\n\t"}`),
		[]byte(`{"type":"ping","type":"join_room"}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > maxWebSocketMessageBytes {
			t.Skip()
		}
		_, _ = decodeWebSocketClientEvent(websocket.MessageText, payload)
	})
}
