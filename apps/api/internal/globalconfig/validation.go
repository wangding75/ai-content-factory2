package globalconfig

import (
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
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
	RepairTarget *RepairTarget `json:"repairTarget,omitempty"`
}

// RepairTarget contains only the safe, routable resource identifiers needed
// to repair an execution-eligibility failure.
type RepairTarget struct {
	ProviderID              *uuid.UUID `json:"providerId,omitempty"`
	ConnectionID            *uuid.UUID `json:"connectionId,omitempty"`
	WorkflowConfigurationID *uuid.UUID `json:"workflowConfigurationId,omitempty"`
	ProjectID               *uuid.UUID `json:"projectId,omitempty"`
	Stage                   *string    `json:"stage,omitempty"`
}

type EligibilityFact struct {
	Kind             string
	ResourceID       *uuid.UUID
	ConnectionID     *uuid.UUID
	WorkflowConfigurationID *uuid.UUID
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
	CredentialRequired  bool
	CredentialAvailable bool
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
			reasons = append(reasons, reason(prefix+"_unverified", "This integration has not been verified.", prefix+":verify", fact))
		case ValidationVerifying:
			reasons = append(reasons, reason(prefix+"_verification_in_progress", "This integration is being verified.", prefix+":view", fact))
		case ValidationFailed:
			reasons = append(reasons, reason(prefix+"_verification_failed", "This integration did not pass verification.", prefix+":verify", fact))
		case ValidationStale:
			reasons = append(reasons, reason(prefix+"_stale", "This integration changed after verification.", prefix+":verify", fact))
		case ValidationVerified:
			if fact.VerifiedVersion == nil || *fact.VerifiedVersion != fact.Version {
				reasons = append(reasons, reason(prefix+"_stale", "The current integration version has not been verified.", prefix+":verify", fact))
			}
		default:
			reasons = append(reasons, reason(prefix+"_unverified", "This integration has not been verified.", prefix+":verify", fact))
		}
		if !fact.Enabled {
			reasons = append(reasons, reason(prefix+"_disabled", "This integration is disabled.", prefix+":enable", fact))
		}
		if fact.CredentialRequired && !fact.CredentialAvailable {
			reasons = append(reasons, reason(prefix+"_credential_unavailable", "The integration credential is unavailable.", prefix+":edit", fact))
		}
		if prefix == "provider" && !fact.ModelAvailable {
			reasons = append(reasons, reason("model_unavailable", "The selected model is unavailable.", "provider:models", fact))
		}
		if prefix == "workflow_configuration" {
			if !fact.StrategyComplete {
				reasons = append(reasons, reason("llm_strategy_incomplete", "The LLM strategy is incomplete.", "workflow_configuration:edit", fact))
			}
			if !fact.ReferenceExists {
				reasons = append(reasons, reason("workflow_reference_not_found", "The workflow reference was not found.", "workflow_configuration:edit", fact))
			}
			if !fact.ReferenceActive {
				reasons = append(reasons, reason("workflow_not_active", "The referenced workflow is not active.", "workflow_configuration:edit", fact))
			}
			if !fact.StageMatches {
				reasons = append(reasons, reason("workflow_stage_mismatch", "The workflow does not support this stage.", "workflow_configuration:edit", fact))
			}
			if !fact.InputCompatible {
				reasons = append(reasons, reason("input_contract_incompatible", "The workflow input contract is incompatible.", "workflow_configuration:edit", fact))
			}
			if !fact.OutputCompatible {
				reasons = append(reasons, reason("output_contract_incompatible", "The workflow output contract is incompatible.", "workflow_configuration:edit", fact))
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

func reason(code, message, repair string, fact EligibilityFact) IneligibilityReason {
	target := &RepairTarget{ConnectionID: fact.ConnectionID, WorkflowConfigurationID: fact.WorkflowConfigurationID}
	switch fact.Kind {
	case "provider": target.ProviderID = fact.ResourceID
	case "connection": target.ConnectionID = fact.ResourceID
	case "workflow_configuration": target.WorkflowConfigurationID = fact.ResourceID
	}
	if target.ProviderID == nil && target.ConnectionID == nil && target.WorkflowConfigurationID == nil {
		target = nil
	}
	return IneligibilityReason{Code: code, Message: message, RepairAction: repair, RepairTarget: target}
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
