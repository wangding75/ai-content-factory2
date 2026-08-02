"""Executable Iteration 19 OpenAPI acceptance checks.

The checks intentionally read the canonical OpenAPI document.  They are not a
second contract source: names and values below are the frozen acceptance
surface that the canonical document must expose.
"""

from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[3]
DOCUMENT = yaml.safe_load((ROOT / "packages/contracts/openapi/openapi.yaml").read_text(encoding="utf-8"))
PATHS = DOCUMENT["paths"]
SCHEMAS = DOCUMENT["components"]["schemas"]
PARAMETERS = DOCUMENT["components"]["parameters"]


def resolve(node):
    while isinstance(node, dict) and "$ref" in node:
        target = DOCUMENT
        for part in node["$ref"][2:].split("/"):
            target = target[part]
        node = target
    return node


def operation(path, method):
    return PATHS[path][method]


def parameter_names(path, method):
    values = PATHS[path].get("parameters", []) + operation(path, method).get("parameters", [])
    return {(resolve(value)["in"], resolve(value)["name"]) for value in values}


def request_schema(path, method):
    return resolve(operation(path, method)["requestBody"])["content"]["application/json"]["schema"]


def assert_command(path, operation_id, *, schema=None):
    current = operation(path, "post")
    assert current["operationId"] == operation_id
    assert ("header", "Idempotency-Key") in parameter_names(path, "post")
    if schema:
        assert request_schema(path, "post")["$ref"].endswith("/" + schema)
        assert "expectedVersion" in SCHEMAS[schema]["required"]
    assert {"409", "422"} <= set(current["responses"])


def test_frozen_enums():
    expected = {
        "ValidationStatus": ["unverified", "verifying", "verified", "failed", "stale"],
        "LlmStrategy": ["acf_managed", "n8n_managed", "none"],
        "RuntimeStatus": ["queued", "running", "cancelling", "succeeded", "failed", "cancelled", "timed_out"],
        "DisplayStatus": ["queued", "running", "cancelling", "succeeded", "failed", "cancelled", "timed_out", "output_validation_failed", "result_consumption_failed"],
        "FailurePhase": ["external_execution", "output_validation", "result_consumption", "cancellation"],
        "RetryMode": ["current_configuration", "original_configuration"],
    }
    for name, values in expected.items():
        assert SCHEMAS[name]["enum"] == values


def test_configuration_commands_and_models():
    assert_command("/api/v1/llm-providers/{providerId}/verify", "verifyLlmProvider", schema="VerifyLlmProviderRequest")
    assert_command("/api/v1/llm-providers/{providerId}/enable", "enableLlmProvider", schema="ConfigurationActionRequest")
    assert_command("/api/v1/llm-providers/{providerId}/disable", "disableLlmProvider", schema="ConfigurationActionRequest")
    assert_command("/api/v1/llm-providers/{providerId}/models/discover", "discoverLlmProviderModels", schema="ConfigurationActionRequest")
    assert operation("/api/v1/llm-providers/{providerId}/models", "get")["operationId"] == "listLlmProviderModels"
    assert_command("/api/v1/workflow-connections/{connectionId}/verify", "verifyWorkflowConnection", schema="ConfigurationActionRequest")
    assert_command("/api/v1/workflow-connections/{connectionId}/enable", "enableWorkflowConnection", schema="ConfigurationActionRequest")
    assert_command("/api/v1/workflow-connections/{connectionId}/disable", "disableWorkflowConnection", schema="ConfigurationActionRequest")
    assert_command("/api/v1/workflow-configurations/{workflowId}/verify", "verifyWorkflowConfiguration", schema="ConfigurationActionRequest")
    assert_command("/api/v1/workflow-configurations/{workflowId}/enable", "enableWorkflowConfiguration", schema="ConfigurationActionRequest")
    assert_command("/api/v1/workflow-configurations/{workflowId}/disable", "disableWorkflowConfiguration", schema="ConfigurationActionRequest")


def test_safe_configuration_resources():
    for name in ("LlmProvider", "WorkflowConnection", "WorkflowConfiguration"):
        properties = SCHEMAS[name]["properties"]
        assert {"validationStatus", "enabled", "executable", "version", "checks", "safeError"} <= set(properties)
        assert properties["validationStatus"].get("readOnly") is True
        assert properties["executable"].get("readOnly") is True
    assert SCHEMAS["CreateLlmProviderRequest"]["properties"]["secret"]["writeOnly"] is True
    assert SCHEMAS["UpdateLlmProviderRequest"]["properties"]["secret"]["writeOnly"] is True
    assert SCHEMAS["CreateWorkflowConnectionRequest"]["properties"]["credential"]["writeOnly"] is True
    assert SCHEMAS["UpdateWorkflowConnectionRequest"]["properties"]["credential"]["writeOnly"] is True
    response_names = {"LlmProvider", "WorkflowConnection", "WorkflowConfiguration", "Iteration14WorkflowRun"}
    forbidden = {"secret", "credential", "authorization", "cookie", "rawResponse", "encryptedSecret", "encryptedCredential"}
    for name in response_names:
        assert not (forbidden & set(SCHEMAS[name].get("properties", {}))), name
    policy = SCHEMAS["WorkflowConfiguration"]["properties"]
    assert {"llmStrategy", "llmProviderId", "llmModel"} <= set(policy)
    binding_request = SCHEMAS["PutProjectWorkflowBindingRequest"]["properties"]
    assert not ({"llmStrategy", "llmProviderId", "llmModel"} & set(binding_request))


def test_binding_and_runtime_recovery():
    candidate_path = "/api/v1/projects/{projectId}/workflow-bindings/{stage}/candidates"
    assert operation(candidate_path, "get")["operationId"] == "listProjectWorkflowBindingCandidates"
    binding = SCHEMAS["WorkflowBindingStage"]["properties"]
    assert {"bound", "selectable", "executable", "ineligibilityReasons", "connectionSummary", "llmPolicySummary"} <= set(binding)
    retry_options_path = "/api/v1/workflow-runs/{runId}/retry-options"
    assert operation(retry_options_path, "get")["operationId"] == "getWorkflowRunRetryOptions"
    assert_command("/api/v1/workflow-runs/{runId}/retries", "retryWorkflowRun", schema="WorkflowRunRetryRequest")
    assert_command("/api/v1/workflow-runs/{runId}/cancel", "cancelWorkflowRun", schema="WorkflowRunCommandRequest")
    assert "mode" in SCHEMAS["WorkflowRunRetryRequest"]["properties"]
    assert "expectedVersion" in SCHEMAS["WorkflowRunRetryRequest"]["required"]
    # New mode and the single deprecated boolean compatibility path are explicit;
    # no omitted boolean zero value can select original_configuration.
    assert len(SCHEMAS["WorkflowRunRetryRequest"]["oneOf"]) == 4
    run = SCHEMAS["Iteration14WorkflowRun"]["properties"]
    frozen_fields = {"workflowName", "workflowConfigurationVersion", "displayStatus", "failurePhase", "failureCode", "safeError", "domainImpact", "connectionSummary", "llmPolicySummary", "retryability", "retryMode", "externalExecutionId", "cancellationRequestedAt", "timedOutAt", "bindingSnapshot", "connectionSnapshot", "llmPolicySnapshot"}
    assert frozen_fields <= set(run)
    events = set(SCHEMAS["WorkflowRunEvent"]["properties"]["eventType"]["enum"])
    assert {"cancel_requested", "timed_out", "output_validation_failed", "result_consumption_failed", "retry_created"} <= events


def test_list_filters_and_shared_safe_schemas():
    run_filters = parameter_names("/api/v1/workflow-runs", "get")
    for name in ("displayStatus", "connectionId", "providerId", "model", "configurationVersion", "retryability", "from", "to", "q", "limit", "offset"):
        assert ("query", name) in run_filters
    for name in ("SafeError", "ValidationCheck", "NonExecutableReason"):
        assert SCHEMAS[name]["additionalProperties"] is False
    layers = resolve(SCHEMAS["WorkflowConfigurationValidationResult"]["properties"]["checksByLayer"])
    assert set(layers["properties"]) == {
        "connection", "workflow_reference", "stage", "input_contract", "output_contract", "llm_strategy"
    }


def test_repair_targets_are_shared_and_safe():
    target = SCHEMAS["RepairTarget"]
    assert target["additionalProperties"] is False
    assert set(target["properties"]) == {"providerId", "connectionId", "workflowConfigurationId", "projectId", "stage"}
    for name in ("providerId", "connectionId", "workflowConfigurationId", "projectId"):
        assert target["properties"][name]["format"] == "uuid"
    assert target["properties"]["stage"]["$ref"].endswith("/ApplicableStage")
    for name in ("ValidationCheck", "NonExecutableReason", "ContentGenerationCheck", "ContentReviewCheck", "ContentRewriteCheck", "ChapterPlanningBlockerItem"):
        assert SCHEMAS[name]["properties"]["repairTarget"]["$ref"].endswith("/RepairTarget")


if __name__ == "__main__":
    for name, value in sorted(globals().copy().items()):
        if name.startswith("test_") and callable(value):
            value()
            print(f"[PASS] {name}")
