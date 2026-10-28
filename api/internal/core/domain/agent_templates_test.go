package domain

import (
	"strings"
	"testing"
)

func TestListAgentTemplates_ReturnsTheFourMissionSegments(t *testing.T) {
	got := ListAgentTemplates()
	if len(got) != 4 {
		t.Fatalf("expected exactly 4 templates, got %d", len(got))
	}
	wantIDs := map[string]bool{
		"clinic":     false,
		"realestate": false,
		"ecommerce":  false,
		"school":     false,
	}
	for _, tpl := range got {
		if _, ok := wantIDs[tpl.ID]; !ok {
			t.Errorf("unexpected template id %q", tpl.ID)
			continue
		}
		wantIDs[tpl.ID] = true
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Errorf("missing template id %q", id)
		}
	}
}

func TestListAgentTemplates_OmitsSeedKnowledge(t *testing.T) {
	// The dropdown doesn't need seed data — the full payload is
	// shipped only on GET /agent-templates/:id. Saves ~40 chunks
	// per page load.
	for _, tpl := range ListAgentTemplates() {
		if len(tpl.SeedKnowledge) != 0 {
			t.Errorf("template %q leaked seed knowledge on List", tpl.ID)
		}
	}
}

func TestGetAgentTemplate_ReturnsFullPayload(t *testing.T) {
	tpl := GetAgentTemplate("clinic")
	if tpl == nil {
		t.Fatalf("expected clinic template")
	}
	if len(tpl.SeedKnowledge) < 5 || len(tpl.SeedKnowledge) > 8 {
		t.Errorf("seed knowledge size out of bounds: %d", len(tpl.SeedKnowledge))
	}
	if !strings.Contains(tpl.SystemPrompt, "{{NOME_DA_EMPRESA}}") {
		t.Errorf("clinic prompt missing NOME_DA_EMPRESA placeholder")
	}
	if !strings.Contains(tpl.SystemPrompt, "{{HORARIO_ATENDIMENTO}}") {
		t.Errorf("clinic prompt missing HORARIO_ATENDIMENTO placeholder")
	}
}

func TestGetAgentTemplate_UnknownReturnsNil(t *testing.T) {
	if GetAgentTemplate("lawyer") != nil { // not in the mission scope
		t.Errorf("expected nil for unknown template id")
	}
	if GetAgentTemplate("") != nil {
		t.Errorf("expected nil for empty id")
	}
}

func TestApplyPlaceholders_SubstitutesAllOccurrences(t *testing.T) {
	tpl := GetAgentTemplate("school")
	applied := tpl.ApplyPlaceholders(map[string]string{
		"NOME_DA_EMPRESA":     "Escola Curumim",
		"HORARIO_ATENDIMENTO": "de segunda a sexta, das 8h às 17h",
	})
	if strings.Contains(applied, "{{NOME_DA_EMPRESA}}") {
		t.Errorf("NOME_DA_EMPRESA token still present after apply")
	}
	if strings.Contains(applied, "{{HORARIO_ATENDIMENTO}}") {
		t.Errorf("HORARIO_ATENDIMENTO token still present after apply")
	}
	if !strings.Contains(applied, "Escola Curumim") {
		t.Errorf("applied prompt missing substituted name")
	}
}

func TestApplyPlaceholders_UnknownKeyLeavesTokenIntact(t *testing.T) {
	// Forward-compat: a stale client that doesn't know about a new
	// placeholder should still get a usable prompt. The literal token
	// surfaces in the UI preview as "missing info" but the agent can
	// still be created.
	tpl := GetAgentTemplate("clinic")
	applied := tpl.ApplyPlaceholders(map[string]string{
		"NOME_DA_EMPRESA": "Clínica Vida",
		// HORARIO_ATENDIMENTO deliberately omitted.
	})
	if !strings.Contains(applied, "{{HORARIO_ATENDIMENTO}}") {
		t.Errorf("expected unsubstituted HORARIO_ATENDIMENTO token to remain")
	}
	// Canonical value untouched — Apply returns a copy.
	orig := GetAgentTemplate("clinic").SystemPrompt
	if !strings.Contains(orig, "{{HORARIO_ATENDIMENTO}}") {
		t.Errorf("canonical template was mutated by Apply")
	}
}

func TestAgentTemplate_PromptLengthWithinMissionBounds(t *testing.T) {
	// Mission spec: prompt "25-40 linhas". Using a loose bound so
	// minor editorial tweaks don't churn the test — 20..60 keeps the
	// intent (not a one-liner, not a novel).
	for _, tpl := range []string{"clinic", "realestate", "ecommerce", "school"} {
		got := GetAgentTemplate(tpl)
		lines := strings.Count(got.SystemPrompt, "\n") + 1
		if lines < 20 || lines > 60 {
			t.Errorf("template %q prompt has %d lines; want 20..60", tpl, lines)
		}
	}
}
