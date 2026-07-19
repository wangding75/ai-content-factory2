Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

@'
from pathlib import Path
import yaml

document = yaml.safe_load(Path("packages/contracts/openapi/openapi.yaml").read_text(encoding="utf-8"))
paths = document["paths"]
schemas = document["components"]["schemas"]

expected = {
    "/api/v1/llm-provider-types": {"get": "listLlmProviderTypes"},
    "/api/v1/llm-providers": {"get": "listLlmProviders", "post": "createLlmProvider"},
    "/api/v1/llm-providers/{providerId}": {"get": "getLlmProvider", "patch": "updateLlmProvider"},
    "/api/v1/workflow-connection-types": {"get": "listWorkflowConnectionTypes"},
    "/api/v1/workflow-connections": {"get": "listWorkflowConnections", "post": "createWorkflowConnection"},
    "/api/v1/workflow-connections/{connectionId}": {"get": "getWorkflowConnection", "patch": "updateWorkflowConnection"},
    "/api/v1/workflow-configurations": {"get": "listWorkflowConfigurations", "post": "createWorkflowConfiguration"},
    "/api/v1/workflow-configurations/{workflowId}": {"get": "getWorkflowConfiguration", "patch": "updateWorkflowConfiguration"},
    "/api/v1/distribution-platform-types": {"get": "listDistributionPlatformTypes"},
    "/api/v1/distribution-platforms": {"get": "listDistributionPlatforms", "post": "createDistributionPlatform"},
    "/api/v1/distribution-platforms/{platformId}": {"get": "getDistributionPlatform", "patch": "updateDistributionPlatform"},
}
for path, methods in expected.items():
    assert path in paths, f"missing Iteration 12 path: {path}"
    actual = {name: value for name, value in paths[path].items() if name in {"get", "post", "patch", "delete"}}
    assert set(actual) == set(methods), f"unexpected methods for {path}: {set(actual)}"
    for method, operation_id in methods.items():
        assert actual[method]["operationId"] == operation_id, f"wrong operationId for {method.upper()} {path}"

for path in paths:
    if path.startswith(("/api/v1/llm-providers", "/api/v1/workflow-connections", "/api/v1/workflow-configurations", "/api/v1/distribution-platforms")):
        assert "delete" not in paths[path], f"Iteration 12 must not expose DELETE: {path}"
        assert not any(part in path for part in ("/verify", "/enable", "/disable", "/models")), f"forbidden Iteration 12 endpoint: {path}"

for operation in ("createLlmProvider", "updateLlmProvider", "createWorkflowConnection", "updateWorkflowConnection", "createWorkflowConfiguration", "updateWorkflowConfiguration", "createDistributionPlatform", "updateDistributionPlatform"):
    found = [value for item in paths.values() for value in item.values() if isinstance(value, dict) and value.get("operationId") == operation]
    assert len(found) == 1, f"operation must be unique: {operation}"
    parameters = found[0].get("parameters", [])
    assert any(parameter.get("$ref") == "#/components/parameters/IdempotencyKey" for parameter in parameters), f"missing idempotency key: {operation}"
    assert set(("400", "409", "422" if operation.startswith("create") or operation.startswith("update") else "400")) <= set(found[0]["responses"]), f"missing write error response: {operation}"

for name in ("UpdateLlmProviderRequest", "UpdateWorkflowConnectionRequest", "UpdateWorkflowConfigurationRequest", "UpdateDistributionPlatformRequest"):
    schema = schemas[name]
    assert schema["required"] == ["expectedVersion"] and schema["minProperties"] == 2, f"optimistic-lock contract drift: {name}"

for name, immutable in (("UpdateLlmProviderRequest", "providerType"), ("UpdateWorkflowConnectionRequest", "connectionType"), ("UpdateDistributionPlatformRequest", "platformType")):
    assert immutable not in schemas[name]["properties"], f"immutable field accepted by PATCH: {name}.{immutable}"

for name, clear_field, secret_field in (("UpdateLlmProviderRequest", "clearSecret", "secret"), ("UpdateWorkflowConnectionRequest", "clearCredential", "credential"), ("UpdateDistributionPlatformRequest", "clearCredential", "credential")):
    properties = schemas[name]["properties"]
    assert properties[clear_field]["const"] is True, f"missing explicit clear semantics: {name}"
    assert properties[secret_field]["minLength"] == 1 and properties[secret_field]["writeOnly"] is True, f"empty or readable credential allowed: {name}"

for name in ("LlmProvider", "WorkflowConnection", "DistributionPlatform"):
    properties = schemas[name]["properties"]
    assert not any(key in {"secret", "credential", "encryptedSecret", "encryptedCredential", "authorization"} for key in properties), f"sensitive material leaked by {name}"
    safe_indicator = "hasSecret" if name == "LlmProvider" else "hasCredential"
    assert properties[safe_indicator]["readOnly"] is True, f"missing safe credential signal: {name}"
    assert properties["integrationStatus"]["readOnly"] is True and properties["enabled"]["readOnly"] is True, f"runtime status mutable in DTO: {name}"

assert schemas["WorkflowConnectionTypeCode"]["enum"] == ["n8n"], "unsupported connection type admitted"
assert "workflowType" not in schemas["CreateWorkflowConfigurationRequest"]["properties"]
assert "workflowType" not in schemas["UpdateWorkflowConfigurationRequest"]["properties"]
assert schemas["WorkflowConfiguration"]["properties"]["workflowType"]["readOnly"] is True
assert schemas["CreateWorkflowConfigurationRequest"]["required"] == ["name", "connectionId", "applicableStages", "typeConfig", "inputContractVersion", "outputContractVersion"]
print("[PASS] Iteration 12 CRUD OpenAPI contract validation completed.")
'@ | python -

if ($LASTEXITCODE -ne 0) {
    throw "Iteration 12 OpenAPI contract validation failed."
}
