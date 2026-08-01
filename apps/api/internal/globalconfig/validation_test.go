package globalconfig

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidationTransitionTable(t *testing.T) {
	for _, test := range []struct {
		name    string
		from    ValidationStatus
		event   ValidationEvent
		want    ValidationStatus
		invalid bool
	}{
		{"new verification", ValidationUnverified, ValidationBegin, ValidationVerifying, false},
		{"successful verification", ValidationVerifying, ValidationSucceed, ValidationVerified, false},
		{"failed verification", ValidationVerifying, ValidationFail, ValidationFailed, false},
		{"key change preserves enabled outside state machine", ValidationVerified, ValidationKeyChanged, ValidationStale, false},
		{"disable keeps validation fact", ValidationVerified, ValidationDisable, ValidationVerified, false},
		{"verified does not become enabled state", ValidationVerified, ValidationSucceed, "", true},
		{"illegal reverse", ValidationVerified, ValidationUnverifiedEventForTest(), "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := TransitionValidation(test.from, test.event)
			if test.invalid {
				if !errors.Is(err, ErrInvalidValidationTransition) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("got=%q err=%v want=%q", got, err, test.want)
			}
		})
	}
}

func ValidationUnverifiedEventForTest() ValidationEvent { return ValidationEvent("reset") }

func TestEvaluateEligibilityStableOrderingAndRecovery(t *testing.T) {
	version := 3
	facts := []EligibilityFact{
		{Kind: "workflow_configuration", Status: ValidationStale, Enabled: true, Version: 3, VerifiedVersion: &version, StrategyComplete: false, ReferenceExists: true, ReferenceActive: true, StageMatches: true, InputCompatible: true, OutputCompatible: true},
		{Kind: "connection", Status: ValidationVerified, Enabled: false, Version: 3, VerifiedVersion: &version, ModelAvailable: true, StrategyComplete: true, ReferenceExists: true, ReferenceActive: true, StageMatches: true, InputCompatible: true, OutputCompatible: true},
		{Kind: "provider", Status: ValidationVerified, Enabled: true, Version: 3, VerifiedVersion: &version, ModelAvailable: false, StrategyComplete: true, ReferenceExists: true, ReferenceActive: true, StageMatches: true, InputCompatible: true, OutputCompatible: true},
	}
	executable, first := EvaluateEligibility(facts...)
	_, second := EvaluateEligibility(facts...)
	if executable || !reflect.DeepEqual(first, second) {
		t.Fatalf("executable=%v first=%+v second=%+v", executable, first, second)
	}
	wantCodes := []string{"workflow_configuration_stale", "connection_disabled", "llm_strategy_incomplete", "model_unavailable"}
	gotCodes := make([]string, len(first))
	for index := range first {
		gotCodes[index] = first[index].Code
		if first[index].RepairAction == "" {
			t.Fatalf("missing repair action: %+v", first[index])
		}
	}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("codes=%v want=%v", gotCodes, wantCodes)
	}
	for index := range facts {
		facts[index].Status = ValidationVerified
		facts[index].Enabled = true
		facts[index].VerifiedVersion = &version
		facts[index].ModelAvailable = true
		facts[index].StrategyComplete = true
	}
	if executable, reasons := EvaluateEligibility(facts...); !executable || len(reasons) != 0 {
		t.Fatalf("recovered executable=%v reasons=%+v", executable, reasons)
	}
}
