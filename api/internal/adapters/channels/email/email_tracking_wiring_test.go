package email

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The loop these tests pin: a send counts against the template version that
// produced it, and tracking is applied ONLY where it doesn't cost more than
// it measures.

type fakeStats struct {
	domain.EmailTemplateRepository
	sends []string
	err   error
}

func (f *fakeStats) RecordSend(_ context.Context, id string, version int) error {
	f.sends = append(f.sends, id+"@"+string(rune('0'+version)))
	return f.err
}

func trackingAdapter(t *testing.T) (*Adapter, *captureTransport, *fakeStats) {
	t.Helper()
	a, cap := newTestAdapter(t)
	stats := &fakeStats{}
	a.SetTracking("https://track.replider.com", "segredo")
	a.SetTemplateStats(stats)
	return a, cap, stats
}

func htmlMsg(templateID string) *domain.Message {
	return &domain.Message{
		Type:      "text",
		Direction: "outbound",
		Content:   str(`<p>Bom dia! Veja a <a href="https://clinica.com.br/proposta">proposta</a>.</p>`),
		Metadata: map[string]any{
			MetaSubject:         "Proposta",
			MetaTemplateID:      templateID,
			MetaTemplateVersion: 2,
		},
	}
}

func TestSend_CountsAgainstTheTemplateVersion(t *testing.T) {
	a, _, stats := trackingAdapter(t)

	err := a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", htmlMsg("tpl1"))

	require.NoError(t, err)
	assert.Equal(t, []string{"tpl1@2"}, stats.sends)
}

// A send that never left is not a send. Counting it would make the template's
// reply rate look worse than the words deserve.
func TestSend_FailedDeliveryIsNotCounted(t *testing.T) {
	a, cap, stats := trackingAdapter(t)
	cap.err = errors.New("smtp: connection refused")

	err := a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", htmlMsg("tpl1"))

	require.Error(t, err)
	assert.Empty(t, stats.sends)
}

// The message is already out by the time the counter runs — a bookkeeping
// failure must not be reported as a failed send.
func TestSend_CounterFailureDoesNotFailTheSend(t *testing.T) {
	a, _, stats := trackingAdapter(t)
	stats.err = errors.New("db down")

	err := a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", htmlMsg("tpl1"))

	assert.NoError(t, err)
}

func TestSend_HTMLBodyGetsThePixelAndWrappedLinks(t *testing.T) {
	a, cap, _ := trackingAdapter(t)

	require.NoError(t, a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", htmlMsg("tpl1")))

	assert.Contains(t, cap.raw, "/t/o/", "open pixel")
	assert.Contains(t, cap.raw, "/t/c/", "wrapped link")
	assert.NotContains(t, cap.raw, "https://clinica.com.br/proposta")
}

// The deliberate hole. Cold outbound is plain text because that is what gets
// replies; a wrapped link there would show a stranger "track.../t/c/eyJ..."
// where the real destination should be, which reads as phishing. Tracking is
// worth less than the reply it would cost.
func TestSend_PlainTextColdEmailIsLeftUntracked(t *testing.T) {
	a, cap, stats := trackingAdapter(t)
	msg := &domain.Message{
		Type:      "text",
		Direction: "outbound",
		Content:   str("Bom dia! Vi a clínica em https://clinica.com.br/proposta e queria conversar."),
		Metadata: map[string]any{
			MetaSubject:         "Conversa rápida",
			MetaTemplateID:      "tpl1",
			MetaTemplateVersion: 2,
		},
	}

	require.NoError(t, a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", msg))

	assert.NotContains(t, cap.raw, "/t/o/")
	assert.NotContains(t, cap.raw, "/t/c/")
	// The destination the reader sees is the real one.
	assert.Contains(t, cap.raw, "https://clinica.com.br/proposta")
	// The SEND is still counted — reply rate works without a pixel.
	assert.Equal(t, []string{"tpl1@2"}, stats.sends)
}

// No tracking domain configured is the default state of a fresh account, and
// it must produce an ordinary email rather than a broken one.
func TestSend_NoTrackingDomainLeavesTheBodyAlone(t *testing.T) {
	a, cap := newTestAdapter(t)
	a.SetTemplateStats(&fakeStats{})

	require.NoError(t, a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", htmlMsg("tpl1")))

	assert.NotContains(t, cap.raw, "/t/o/")
	assert.Contains(t, cap.raw, "https://clinica.com.br/proposta")
}

// A message written from scratch has no template, and there is nothing to
// credit — the send must still work.
func TestSend_NoTemplateIsNotCounted(t *testing.T) {
	a, _, stats := trackingAdapter(t)
	msg := htmlMsg("")

	require.NoError(t, a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", msg))

	assert.Empty(t, stats.sends)
}

// Metadata that came back from Postgres is raw JSON, where an int is a
// float64. A version silently read as 0 would credit a version that never
// existed, and the real one would show zero sends forever.
func TestSend_TemplateVersionSurvivesAJSONRoundTrip(t *testing.T) {
	a, _, stats := trackingAdapter(t)
	msg := htmlMsg("tpl1")
	msg.Metadata = map[string]any{
		MetaSubject:         "Proposta",
		MetaTemplateID:      "tpl1",
		MetaTemplateVersion: float64(3),
	}

	require.NoError(t, a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", msg))

	assert.Equal(t, []string{"tpl1@3"}, stats.sends)
}

// An unsubscribe link routed through a tracker that is down is a complaint to
// the mailbox provider, not a lost metric.
func TestSend_UnsubscribeLinkIsNeverWrapped(t *testing.T) {
	a, cap, _ := trackingAdapter(t)
	msg := htmlMsg("tpl1")
	msg.Content = str(`<p>Texto. <a href="https://clinica.com.br/descadastrar">sair</a></p>`)

	require.NoError(t, a.SendMessage(context.Background(),
		emailChannel(validConfig()), "dono@clinica.com.br", msg))

	body := strings.ReplaceAll(cap.raw, "=\r\n", "")
	assert.Contains(t, body, "https://clinica.com.br/descadastrar")
}
