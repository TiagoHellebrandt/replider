package realtime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordingDeliverer captures DeliverLocal calls for assertions.
type recordingDeliverer struct {
	calls []struct {
		accountID string
		payload   []byte
	}
}

func (r *recordingDeliverer) DeliverLocal(accountID string, payload []byte) {
	r.calls = append(r.calls, struct {
		accountID string
		payload   []byte
	}{accountID, payload})
}

// The envelope must round-trip the account id and the raw payload bytes
// (json encodes []byte as base64), so a dashboard event survives the hop.
func TestEnvelope_RoundTrip(t *testing.T) {
	payload := []byte(`{"event":"new_message","conversation_id":"conv-1"}`)
	data, err := json.Marshal(envelope{Origin: "pod-a", AccountID: "acc-1", Payload: payload})
	assert.NoError(t, err)

	var got envelope
	assert.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, "pod-a", got.Origin)
	assert.Equal(t, "acc-1", got.AccountID)
	assert.Equal(t, payload, got.Payload)
}

// dispatch mirrors RedisFanout.Run's per-message handling so the skip-self and
// delivery logic is testable without a live Redis broker.
func dispatch(instance string, local LocalDeliverer, raw []byte) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}
	if env.Origin == instance {
		return
	}
	local.DeliverLocal(env.AccountID, env.Payload)
}

func TestDispatch_SkipsOwnMessages(t *testing.T) {
	rd := &recordingDeliverer{}
	raw, _ := json.Marshal(envelope{Origin: "pod-self", AccountID: "acc-1", Payload: []byte("x")})

	dispatch("pod-self", rd, raw)

	assert.Empty(t, rd.calls, "a pod must not redeliver its own published broadcast")
}

func TestDispatch_DeliversPeerMessages(t *testing.T) {
	rd := &recordingDeliverer{}
	payload := []byte(`{"event":"conversation_updated"}`)
	raw, _ := json.Marshal(envelope{Origin: "pod-other", AccountID: "acc-9", Payload: payload})

	dispatch("pod-self", rd, raw)

	assert.Len(t, rd.calls, 1)
	assert.Equal(t, "acc-9", rd.calls[0].accountID)
	assert.Equal(t, payload, rd.calls[0].payload)
}

func TestDispatch_IgnoresMalformedEnvelope(t *testing.T) {
	rd := &recordingDeliverer{}
	dispatch("pod-self", rd, []byte("not-json"))
	assert.Empty(t, rd.calls)
}
