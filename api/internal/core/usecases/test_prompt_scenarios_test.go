package usecases

import (
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestParseJudgeOutput_StrictJSON(t *testing.T) {
	passed, fb := parseJudgeOutput(`{"passed": true, "feedback": "good"}`)
	assert.True(t, passed)
	assert.Equal(t, "good", fb)
}

func TestParseJudgeOutput_FencedJSON(t *testing.T) {
	passed, fb := parseJudgeOutput("```json\n{\"passed\": false, \"feedback\": \"diagnosed\"}\n```")
	assert.False(t, passed)
	assert.Equal(t, "diagnosed", fb)
}

func TestParseJudgeOutput_PreambleAndJSON(t *testing.T) {
	passed, fb := parseJudgeOutput(`Sure, here is the verdict: {"passed": true, "feedback": "redirected"}`)
	assert.True(t, passed)
	assert.Equal(t, "redirected", fb)
}

func TestParseJudgeOutput_Malformed(t *testing.T) {
	passed, fb := parseJudgeOutput("not json at all")
	assert.False(t, passed)
	assert.Contains(t, fb, "malformed")
}

func TestScenariosFor_PrefersUserScenarios(t *testing.T) {
	uc := &TestPromptScenariosUseCase{}
	a := domain.WizardAnswers{
		Vertical:        "clinic",
		CommonScenarios: []string{"my custom scenario"},
	}
	got := uc.scenariosFor("clinic", a)
	assert.Equal(t, []string{"my custom scenario"}, got)
}

func TestScenariosFor_FallsBackToVerticalDefaults(t *testing.T) {
	uc := &TestPromptScenariosUseCase{}
	got := uc.scenariosFor("clinic", domain.WizardAnswers{Vertical: "clinic"})
	// Clinic seeds 5 default scenarios.
	assert.Equal(t, 5, len(got))
}

func TestScenariosFor_ClampsToFive(t *testing.T) {
	uc := &TestPromptScenariosUseCase{}
	a := domain.WizardAnswers{
		CommonScenarios: []string{"a", "b", "c", "d", "e", "f", "g"},
	}
	got := uc.scenariosFor("other", a)
	assert.Equal(t, 5, len(got))
}
