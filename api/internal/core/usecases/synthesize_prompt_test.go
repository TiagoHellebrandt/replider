package usecases

import (
	"strings"
	"testing"

	"github.com/replider/api/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestMergeVerticalDefaults_FillsBlanksOnly(t *testing.T) {
	a := &domain.WizardAnswers{
		Vertical: "clinic",
		Tone:     "cordial",
		ScopeIn:  []string{"agendamento"},
	}
	mergeVerticalDefaults(a)

	// Clinic defaults seed scope_out + compliance + business hours.
	assert.NotEmpty(t, a.ScopeOut, "scope_out should be filled from clinic defaults")
	assert.Contains(t, a.ComplianceFlags, "medical_no_diagnosis")
	assert.NotNil(t, a.BusinessHours)
	assert.Equal(t, "America/Sao_Paulo", a.BusinessHours.Timezone)
	assert.Equal(t, "none", a.EmojiUsage)
}

func TestMergeVerticalDefaults_PreservesUserValues(t *testing.T) {
	customHours := &domain.BusinessHours{WeekdaysFrom: "10:00", Timezone: "UTC"}
	a := &domain.WizardAnswers{
		Vertical:        "clinic",
		Tone:            "cordial",
		ScopeIn:         []string{"x"},
		ScopeOut:        []string{"only this"},
		ComplianceFlags: []string{"custom_flag"},
		BusinessHours:   customHours,
		EmojiUsage:      "liberal",
	}
	mergeVerticalDefaults(a)

	assert.Equal(t, []string{"only this"}, a.ScopeOut)
	assert.Equal(t, []string{"custom_flag"}, a.ComplianceFlags)
	assert.Equal(t, "10:00", a.BusinessHours.WeekdaysFrom)
	assert.Equal(t, "liberal", a.EmojiUsage)
}

func TestBuildSynthesisUserPayload_RendersAllPhases(t *testing.T) {
	a := &domain.WizardAnswers{
		Vertical:        "clinic",
		Tone:            "cordial",
		ScopeIn:         []string{"agendamento"},
		CompanyName:     "Clínica Sol",
		BotName:         "Maria",
		BotPronoun:      "ela",
		EmojiUsage:      "none",
		ServiceStrategy: "consultative",
		ComplianceFlags: []string{"medical_no_diagnosis"},
	}
	payload := buildSynthesisUserPayload(a)

	for _, want := range []string{
		"[IDENTITY]",
		"Company name: Clínica Sol",
		"[PERSONA]",
		"Bot name: Maria",
		"[TONE]",
		"Emoji posture: none",
		"[STRATEGY]",
		"Service strategy: consultative",
		"Compliance flags (HARD must-not): medical_no_diagnosis",
	} {
		assert.True(t, strings.Contains(payload, want), "payload missing %q", want)
	}
}

func TestBuildSynthesisUserPayload_OmitsEmptyFields(t *testing.T) {
	a := &domain.WizardAnswers{
		Vertical: "other",
		Tone:     "cordial",
		ScopeIn:  []string{"x"},
	}
	payload := buildSynthesisUserPayload(a)
	// We never emit "Company name:" when company_name is blank.
	assert.False(t, strings.Contains(payload, "Company name:"))
	assert.False(t, strings.Contains(payload, "Bot name:"))
}
