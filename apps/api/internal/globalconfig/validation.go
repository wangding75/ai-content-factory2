package globalconfig

import (
	"errors"
	"sort"
	"strings"
)

type ValidationStatus string
type ValidationEvent string

const (
	ValidationUnverified ValidationStatus = "unverified"
	ValidationVerifying  ValidationStatus = "verifying"
	ValidationVerified   ValidationStatus = "verified"
	ValidationFailed     ValidationStatus = "failed"
	ValidationStale      ValidationStatus = "stale"

	ValidationBegin      ValidationEvent = "begin"
	ValidationSucceed    ValidationEvent = "succeed"
	ValidationFail       ValidationEvent = "fail"
	ValidationKeyChanged ValidationEvent = "key_changed"
	ValidationDisable    ValidationEvent = "disable"
)

var ErrInvalidValidationTransition = errors.New("invalid validation status transition")

func TransitionValidation(current ValidationStatus, event ValidationEvent) (ValidationStatus, error) {
	if event == ValidationDisable {
		return current, nil
	}
	if event == ValidationKeyChanged {
		if validValidationStatus(current) {
			return ValidationStale, nil
		}
		return "", ErrInvalidValidationTransition
	}
	switch {
	case event == ValidationBegin && (current == ValidationUnverified || current == ValidationFailed || current == ValidationStale || current == ValidationVerified):
		return ValidationVerifying, nil
	case event == ValidationSucceed && current == ValidationVerifying:
		return ValidationVerified, nil
	case event == ValidationFail && current == ValidationVerifying:
		return ValidationFailed, nil
	default:
		return "", ErrInvalidValidationTransition
	}
}

func validValidationStatus(status ValidationStatus) bool {
	return status == ValidationUnverified || status == ValidationVerifying || status == ValidationVerified || status == ValidationFailed || status == ValidationStale
}

type IneligibilityReason struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	RepairAction string `json:"repairAction,omitempty"`
}

type EligibilityFact struct {
	Kind             string
	Status           ValidationStatus
	Enabled          bool
	Version          int
	VerifiedVersion  *int
	ModelAvailable   bool
	StrategyComplete bool
	ReferenceExists  bool
	ReferenceActive  bool
	StageMatches     bool
	InputCompatible  bool
	OutputCompatible bool
}

func EvaluateEligibility(facts ...EligibilityFact) (bool, []IneligibilityReason) {
	reasons := make([]IneligibilityReason, 0)
	for _, fact := range facts {
		prefix := strings.TrimSpace(fact.Kind)
		if prefix == "" {
			prefix = "configuration"
		}
		switch fact.Status {
		case ValidationUnverified:
			reasons = append(reasons, reason(prefix+"_unverified", "This integration has not been verified.", prefix+":verify"))
		case ValidationVerifying:
			reasons = append(reasons, reason(prefix+"_verification_in_progress", "This integration is being verified.", prefix+":view"))
		case ValidationFailed:
			reasons = append(reasons, reason(prefix+"_verification_failed", "This integration did not pass verification.", prefix+":verify"))
		case ValidationStale:
			reasons = append(reasons, reason(prefix+"_stale", "This integration changed after verification.", prefix+":verify"))
		case ValidationVerified:
			if fact.VerifiedVersion == nil || *fact.VerifiedVersion != fact.Version {
				reasons = append(reasons, reason(prefix+"_stale", "The current integration version has not been verified.", prefix+":verify"))
			}
		default:
			reasons = append(reasons, reason(prefix+"_unverified", "This integration has not been verified.", prefix+":verify"))
		}
		if !fact.Enabled {
			reasons = append(reasons, reason(prefix+"_disabled", "This integration is disabled.", prefix+":enable"))
		}
		if prefix == "provider" && !fact.ModelAvailable {
			reasons = append(reasons, reason("model_unavailable", "The selected model is unavailable.", "provider:models"))
		}
		if prefix == "workflow_configuration" {
			if !fact.StrategyComplete {
				reasons = append(reasons, reason("llm_strategy_incomplete", "The LLM strategy is incomplete.", "workflow_configuration:edit"))
			}
			if !fact.ReferenceExists {
				reasons = append(reasons, reason("workflow_reference_not_found", "The workflow reference was not found.", "workflow_configuration:edit"))
			}
			if !fact.ReferenceActive {
				reasons = append(reasons, reason("workflow_not_active", "The referenced workflow is not active.", "workflow_configuration:edit"))
			}
			if !fact.StageMatches {
				reasons = append(reasons, reason("workflow_stage_mismatch", "The workflow does not support this stage.", "workflow_configuration:edit"))
			}
			if !fact.InputCompatible {
				reasons = append(reasons, reason("input_contract_incompatible", "The workflow input contract is incompatible.", "workflow_configuration:edit"))
			}
			if !fact.OutputCompatible {
				reasons = append(reasons, reason("output_contract_incompatible", "The workflow output contract is incompatible.", "workflow_configuration:edit"))
			}
		}
	}
	sort.SliceStable(reasons, func(i, j int) bool {
		if reasonPriority(reasons[i].Code) == reasonPriority(reasons[j].Code) {
			return reasons[i].Code < reasons[j].Code
		}
		return reasonPriority(reasons[i].Code) < reasonPriority(reasons[j].Code)
	})
	return len(reasons) == 0, reasons
}

func reason(code, message, repair string) IneligibilityReason {
	return IneligibilityReason{Code: code, Message: message, RepairAction: repair}
}
func reasonPriority(code string) int {
	switch {
	case strings.Contains(code, "unverified"), strings.Contains(code, "verification_in_progress"), strings.Contains(code, "verification_failed"), strings.HasSuffix(code, "_stale"):
		return 10
	case strings.HasSuffix(code, "_disabled"):
		return 20
	case code == "model_unavailable", code == "llm_strategy_incomplete":
		return 30
	default:
		return 40
	}
}
