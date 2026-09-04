package protocol

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzClientEventJSON(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(``),
		[]byte(`null`),
		[]byte(`{`),
		[]byte(`[]`),
		[]byte(`{"type":"ping","request_id":"seed-1"}`),
		[]byte(`{"type":1}`),
		[]byte(`{"type":"ping","type":"unknown"}`),
		[]byte(`{"type":"send_message","content":"\u0000\u001b[31m\u202e"}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 16*1024 {
			t.Skip()
		}

		var event ClientEvent
		decoder := json.NewDecoder(bytes.NewReader(payload))
		_ = decoder.Decode(&event)
	})
}
