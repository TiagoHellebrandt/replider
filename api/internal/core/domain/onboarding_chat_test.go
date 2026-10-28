package domain

import (
	"testing"
	"time"
)

// Conversational onboarding (K5).
//
// The ladder is the product decision here: fixed order, one required answer, and
// an activation metric that refuses to flatter itself.

func TestNextOnboardingStep_WalksTheLadder(t *testing.T) {
	want := map[OnboardingStep]OnboardingStep{
		StepCompany:   StepProduct,
		StepProduct:   StepAudiences,
		StepAudiences: StepChannel,
		StepChannel:   StepDone,
		StepDone:      StepDone,
	}
	for from, to := range want {
		if got := NextOnboardingStep(from); got != to {
			t.Fatalf("%s → %s, want %s", from, got, to)
		}
	}
}

func TestNextOnboardingStep_ACorruptedValueRestartsRatherThanFinishes(t *testing.T) {
	// Silently declaring the conversation done would leave an account with
	// nothing configured and no way back into onboarding.
	if got := NextOnboardingStep(OnboardingStep("whatever")); got != StepCompany {
		t.Fatalf("unknown step → %s, want the first step", got)
	}
}

func TestCanAdvance_OnlyTheCompanyStepBlocks(t *testing.T) {
	empty := &OnboardingChat{Step: StepCompany}
	if empty.CanAdvance() {
		t.Fatal("an agent with no company introduces itself as nobody")
	}

	named := &OnboardingChat{Step: StepCompany,
		Answers: OnboardingAnswers{CompanyName: "Clínica Bem Viver"}}
	if !named.CanAdvance() {
		t.Fatal("a named company must advance")
	}

	// Everything else is skippable and fillable later from its own screen. An
	// onboarding that refuses to move because nobody has decided on a price is
	// an onboarding people abandon at the price question.
	for _, step := range []OnboardingStep{StepProduct, StepAudiences, StepChannel} {
		chat := &OnboardingChat{Step: step}
		if !chat.CanAdvance() {
			t.Fatalf("%s must be skippable", step)
		}
	}
}

func TestTimeToFirstLead_IsNilUntilThereIsALead(t *testing.T) {
	started := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	inProgress := &OnboardingChat{StartedAt: started}

	// An account still in its first hour is not an account whose
	// time-to-first-lead is one hour. Counting the in-progress ones would make
	// the metric improve every time somebody signs up and does nothing.
	if inProgress.TimeToFirstLead() != nil {
		t.Fatal("no lead yet means no measurement")
	}

	landed := started.Add(7 * time.Minute)
	activated := &OnboardingChat{StartedAt: started, FirstLeadAt: &landed}
	got := activated.TimeToFirstLead()
	if got == nil || *got != 7*time.Minute {
		t.Fatalf("time-to-first-lead = %v, want 7m", got)
	}
}

func TestTimeToFirstLead_RefusesToGoNegative(t *testing.T) {
	started := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	earlier := started.Add(-time.Hour)
	chat := &OnboardingChat{StartedAt: started, FirstLeadAt: &earlier}

	// An account that arrived here after already prospecting. Reporting a
	// negative activation time is worse than reporting none.
	if chat.TimeToFirstLead() != nil {
		t.Fatal("a lead predating the conversation must not report a negative time")
	}
}

func TestPromptFor_AsksAboutTheCompanyByName(t *testing.T) {
	prompt := PromptFor(StepProduct, OnboardingAnswers{CompanyName: "Clínica Bem Viver"})

	// Using the name they just gave is the cheapest possible proof the
	// conversation is listening.
	if !contains(prompt.Text, "Clínica Bem Viver") {
		t.Fatalf("prompt %q does not use the company name", prompt.Text)
	}
	if !prompt.Skippable {
		t.Fatal("the product step is skippable")
	}

	// With no name yet it still has to read as a sentence.
	anonymous := PromptFor(StepProduct, OnboardingAnswers{})
	if contains(anonymous.Text, "  ") || anonymous.Text == "" {
		t.Fatalf("prompt with no company name reads badly: %q", anonymous.Text)
	}
}

func TestPromptFor_TheCompanyStepIsNotSkippable(t *testing.T) {
	if PromptFor(StepCompany, OnboardingAnswers{}).Skippable {
		t.Fatal("the company step must not offer a skip the server would refuse")
	}
}

func TestPromptFor_EveryStepNamesAWidget(t *testing.T) {
	for _, step := range onboardingLadder {
		prompt := PromptFor(step, OnboardingAnswers{})
		if prompt.Widget == "" {
			t.Fatalf("%s has no widget — the client would not know what to draw", step)
		}
		if prompt.Step != step {
			t.Fatalf("prompt for %s reports step %s", step, prompt.Step)
		}
	}
}

func TestValidOnboardingStep(t *testing.T) {
	if !ValidOnboardingStep("company") || !ValidOnboardingStep("done") {
		t.Fatal("ladder steps must validate")
	}
	if ValidOnboardingStep("payment") {
		t.Fatal("a step outside the ladder must be refused")
	}
}
