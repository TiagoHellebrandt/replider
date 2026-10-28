package e2e

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US-F216. The A/B is judged by whether a human approved the draft WITHOUT
// editing it — reply rate would need seventeen months at pilot volume.

func startExperiment(t *testing.T, token, funnelID string, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	w := doRequest("POST", "/v1/funnels/"+funnelID+"/approach-experiments", body, token)
	require.Equal(t, 201, w.Code, "create experiment: %s", w.Body.String())
	return parseJSON(t, w)
}

func TestE2E_ApproachAB_RefusesASecondRunningExperiment(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	startExperiment(t, token, funnelID, map[string]interface{}{
		"name": "tom consultivo", "variant_b_suffix": "Seja mais direto.",
	})

	// Two at once make the result unattributable: a lift could belong to
	// either, and "we do not know" afterwards is worse than not running them.
	w := doRequest("POST", "/v1/funnels/"+funnelID+"/approach-experiments",
		map[string]interface{}{"name": "outro", "variant_b_suffix": "Outra coisa."}, token)
	assert.GreaterOrEqual(t, w.Code, 400, "aceitou dois experimentos no mesmo funil")
	assert.Contains(t, w.Body.String(), "EXPERIMENT_ALREADY_RUNNING")
}

// Identical arms would burn the pilot's whole lead budget proving a thing
// equals itself.
func TestE2E_ApproachAB_RefusesIdenticalVariants(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	w := doRequest("POST", "/v1/funnels/"+funnelID+"/approach-experiments",
		map[string]interface{}{"name": "igual", "variant_a_suffix": "x", "variant_b_suffix": "x"}, token)
	assert.GreaterOrEqual(t, w.Code, 400)
	assert.Contains(t, w.Body.String(), "EXPERIMENT_VARIANTS_IDENTICAL")
}

// The verdict must refuse to crown a winner on a sample too small to support
// one — a UI that names a winner invites someone to act on noise.
func TestE2E_ApproachAB_ReportStaysQuietOnASmallSample(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "sdr@test.com", "pass1234")
	verifyEmail(t, "sdr@test.com")
	token := login(t, "sdr@test.com", "pass1234")
	funnelID := createFunnel(t, token, "Clínicas SP")

	exp := startExperiment(t, token, funnelID, map[string]interface{}{
		"name": "tom", "variant_b_suffix": "Seja mais direto.",
	})
	id := exp["id"].(string)

	w := doRequest("GET", "/v1/approach-experiments/"+id, nil, token)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var rep struct {
		Verdict string `json:"verdict"`
		Arms    []struct {
			Variant string `json:"variant"`
		} `json:"arms"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rep))
	assert.Empty(t, rep.Arms, "nenhum rascunho ainda")
	assert.NotContains(t, rep.Verdict, "à frente", "coroou vencedor sem dado: %q", rep.Verdict)
}

// An experiment belongs to its account and to nobody else.
func TestE2E_ApproachAB_IsAccountScoped(t *testing.T) {
	cleanDB(t)
	registerUser(t, "Clinic Co", "owner@test.com", "pass1234")
	verifyEmail(t, "owner@test.com")
	ownerToken := login(t, "owner@test.com", "pass1234")
	funnelID := createFunnel(t, ownerToken, "Clínicas SP")
	exp := startExperiment(t, ownerToken, funnelID, map[string]interface{}{
		"name": "tom", "variant_b_suffix": "Seja mais direto.",
	})

	registerUser(t, "Other Co", "stranger@test.com", "pass1234")
	verifyEmail(t, "stranger@test.com")
	strangerToken := login(t, "stranger@test.com", "pass1234")

	w := doRequest("GET", "/v1/approach-experiments/"+exp["id"].(string), nil, strangerToken)
	assert.NotEqual(t, 200, w.Code, "estranho leu o experimento de outra conta")
}
