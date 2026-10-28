package email

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"
)

const testWebhookSecret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"

var testNow = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

// sign builds what the provider would send, so the test exercises the real
// scheme rather than a restatement of the implementation.
func sign(t *testing.T, secret, id string, ts int64, body []byte) string {
	t.Helper()
	key := decodeWebhookSecret(secret)
	if len(key) == 0 {
		t.Fatalf("test secret does not decode")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func headersFrom(m map[string]string) func(string) string {
	return func(name string) string { return m[name] }
}

func TestWebhookSignature_AcceptsWhatTheProviderSends(t *testing.T) {
	body := []byte(`{"from":"compras@hospital.com","subject":"Proposta"}`)
	ts := testNow.Unix()
	h := headersFrom(map[string]string{
		"svix-id":        "msg_1",
		"svix-timestamp": strconv.FormatInt(ts, 10),
		"svix-signature": sign(t, testWebhookSecret, "msg_1", ts, body),
	})

	if !VerifyWebhookSignature(testWebhookSecret, h, body, testNow) {
		t.Fatal("a correctly signed payload was refused")
	}
}

// Standard Webhooks ships two header spellings; an operator on either provider
// must not be silently unprotected.
func TestWebhookSignature_AcceptsTheOtherHeaderSpelling(t *testing.T) {
	body := []byte(`{"from":"a@b.com"}`)
	ts := testNow.Unix()
	h := headersFrom(map[string]string{
		"webhook-id":        "msg_2",
		"webhook-timestamp": strconv.FormatInt(ts, 10),
		"webhook-signature": sign(t, testWebhookSecret, "msg_2", ts, body),
	})

	if !VerifyWebhookSignature(testWebhookSecret, h, body, testNow) {
		t.Fatal("webhook-* headers were refused")
	}
}

// The whole point: a body nobody signed must not reach the inbox. This is the
// forged supplier e-mail the risk layer would otherwise grade as clean.
func TestWebhookSignature_RefusesAForgedBody(t *testing.T) {
	ts := testNow.Unix()
	signed := []byte(`{"from":"compras@fornecedor.com","body":"conta antiga"}`)
	forged := []byte(`{"from":"compras@fornecedor.com","body":"conta NOVA"}`)
	h := headersFrom(map[string]string{
		"svix-id":        "msg_3",
		"svix-timestamp": strconv.FormatInt(ts, 10),
		"svix-signature": sign(t, testWebhookSecret, "msg_3", ts, signed),
	})

	if VerifyWebhookSignature(testWebhookSecret, h, forged, testNow) {
		t.Fatal("a body swapped after signing was accepted")
	}
}

func TestWebhookSignature_RefusesAnotherSecret(t *testing.T) {
	body := []byte(`{"from":"a@b.com"}`)
	ts := testNow.Unix()
	other := "whsec_" + base64.StdEncoding.EncodeToString([]byte("a different key entirely"))
	h := headersFrom(map[string]string{
		"svix-id":        "msg_4",
		"svix-timestamp": strconv.FormatInt(ts, 10),
		"svix-signature": sign(t, other, "msg_4", ts, body),
	})

	if VerifyWebhookSignature(testWebhookSecret, h, body, testNow) {
		t.Fatal("a signature from another secret was accepted")
	}
}

// A request lifted from a proxy log must not work tomorrow — the timestamp is
// inside the MAC precisely so this window can be enforced.
func TestWebhookSignature_RefusesAReplayOutsideTheWindow(t *testing.T) {
	body := []byte(`{"from":"a@b.com"}`)
	for _, drift := range []time.Duration{
		-(webhookToleranceSeconds + 60) * time.Second,
		(webhookToleranceSeconds + 60) * time.Second, // a future stamp is as forged as a stale one
	} {
		ts := testNow.Add(drift).Unix()
		h := headersFrom(map[string]string{
			"svix-id":        "msg_5",
			"svix-timestamp": strconv.FormatInt(ts, 10),
			"svix-signature": sign(t, testWebhookSecret, "msg_5", ts, body),
		})
		if VerifyWebhookSignature(testWebhookSecret, h, body, testNow) {
			t.Fatalf("a %v old/ahead request was accepted", drift)
		}
	}
	// And the edge inside the window still works, so the bound is a window
	// and not an accident.
	ts := testNow.Add(-(webhookToleranceSeconds - 10) * time.Second).Unix()
	h := headersFrom(map[string]string{
		"svix-id":        "msg_5",
		"svix-timestamp": strconv.FormatInt(ts, 10),
		"svix-signature": sign(t, testWebhookSecret, "msg_5", ts, body),
	})
	if !VerifyWebhookSignature(testWebhookSecret, h, body, testNow) {
		t.Fatal("a fresh request inside the window was refused")
	}
}

// Every missing input fails CLOSED. An unconfigured secret must never mean
// "let it through" — that is the shape the antivirus gate already refuses.
func TestWebhookSignature_MissingAnythingRefuses(t *testing.T) {
	body := []byte(`{"from":"a@b.com"}`)
	ts := strconv.FormatInt(testNow.Unix(), 10)
	good := sign(t, testWebhookSecret, "msg_6", testNow.Unix(), body)

	cases := map[string]struct {
		secret  string
		headers map[string]string
	}{
		"no secret":        {"", map[string]string{"svix-id": "msg_6", "svix-timestamp": ts, "svix-signature": good}},
		"garbage secret":   {"whsec_not-base64!!", map[string]string{"svix-id": "msg_6", "svix-timestamp": ts, "svix-signature": good}},
		"no id":            {testWebhookSecret, map[string]string{"svix-timestamp": ts, "svix-signature": good}},
		"no timestamp":     {testWebhookSecret, map[string]string{"svix-id": "msg_6", "svix-signature": good}},
		"no signature":     {testWebhookSecret, map[string]string{"svix-id": "msg_6", "svix-timestamp": ts}},
		"empty headers":    {testWebhookSecret, map[string]string{}},
		"bad timestamp":    {testWebhookSecret, map[string]string{"svix-id": "msg_6", "svix-timestamp": "ontem", "svix-signature": good}},
		"unknown version":  {testWebhookSecret, map[string]string{"svix-id": "msg_6", "svix-timestamp": ts, "svix-signature": "v9,abcd"}},
		"not base64 sig":   {testWebhookSecret, map[string]string{"svix-id": "msg_6", "svix-timestamp": ts, "svix-signature": "v1,!!!!"}},
		"no version comma": {testWebhookSecret, map[string]string{"svix-id": "msg_6", "svix-timestamp": ts, "svix-signature": "abcd"}},
	}
	for name, tc := range cases {
		if VerifyWebhookSignature(tc.secret, headersFrom(tc.headers), body, testNow) {
			t.Errorf("%s: accepted", name)
		}
	}
	// And a nil header accessor, which is what an unwired caller looks like.
	if VerifyWebhookSignature(testWebhookSecret, nil, body, testNow) {
		t.Error("nil headers: accepted")
	}
}

// Rotation: the provider sends both signatures while the old secret drains.
func TestWebhookSignature_AcceptsOneOfSeveralDuringRotation(t *testing.T) {
	body := []byte(`{"from":"a@b.com"}`)
	ts := testNow.Unix()
	old := "whsec_" + base64.StdEncoding.EncodeToString([]byte("the previous key"))
	h := headersFrom(map[string]string{
		"svix-id":        "msg_7",
		"svix-timestamp": strconv.FormatInt(ts, 10),
		"svix-signature": sign(t, old, "msg_7", ts, body) + " " + sign(t, testWebhookSecret, "msg_7", ts, body),
	})

	if !VerifyWebhookSignature(testWebhookSecret, h, body, testNow) {
		t.Fatal("our signature was refused when it was not the first in the list")
	}
}

// The prefix is optional on OUR side: an operator who pasted the secret
// without `whsec_` gets a working webhook, not a silently open one.
func TestWebhookSignature_PrefixIsOptional(t *testing.T) {
	body := []byte(`{"from":"a@b.com"}`)
	ts := testNow.Unix()
	bare := "MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	h := headersFrom(map[string]string{
		"svix-id":        "msg_8",
		"svix-timestamp": strconv.FormatInt(ts, 10),
		"svix-signature": sign(t, testWebhookSecret, "msg_8", ts, body),
	})

	if !VerifyWebhookSignature(bare, h, body, testNow) {
		t.Fatal("the same secret without the prefix was refused")
	}
}
