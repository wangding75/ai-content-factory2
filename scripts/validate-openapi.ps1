Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

npx.cmd --yes @redocly/cli@1.34.5 lint --skip-rule operation-summary --skip-rule security-defined --skip-rule operation-4xx-response --skip-rule info-license --skip-rule no-server-example.com packages/contracts/openapi/openapi.yaml
if ($LASTEXITCODE -ne 0) {
    throw "OpenAPI validation failed."
}
$openApiText = Get-Content -Raw "packages/contracts/openapi/openapi.yaml"
$iteration04Operations = @(
    "listProjectStorylines",
    "createProjectStoryline",
    "createStorylineChild",
    "updateStoryline",
    "listProjectForeshadowings",
    "createProjectForeshadowing",
    "updateForeshadowing"
)
foreach ($operationId in $iteration04Operations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) {
        throw "Iteration 04 OpenAPI operation is missing: $operationId"
    }
}
$schemaPaths = @(
    "packages/contracts/content-packs/novel/project-planning.schema.json",
    "packages/contracts/content-packs/novel/material.schema.json",
    "packages/contracts/content-packs/novel/plot-line.schema.json",
    "packages/contracts/content-packs/novel/foreshadowing.schema.json",
    "packages/contracts/content-packs/novel/chapter-plan.schema.json",
    "packages/contracts/content-packs/novel/content-item.schema.json",
    "packages/contracts/content-packs/novel/content-version.schema.json",
    "packages/contracts/content-packs/novel/mock-generation-parameters.schema.json",
    "packages/contracts/content-packs/novel/review-report.schema.json",
    "packages/contracts/content-packs/novel/review-finding.schema.json",
    "packages/contracts/content-packs/novel/review-recommendation.schema.json",
    "packages/contracts/content-packs/novel/workflow-run-summary.schema.json",
    "packages/contracts/content-packs/novel/mock-rewrite.schema.json",
    "packages/contracts/content-packs/novel/content-version-query.schema.json",
    "packages/contracts/content-packs/novel/project-work.schema.json"
    ,"packages/contracts/content-packs/novel/global-lite.schema.json"
    ,"packages/contracts/content-packs/novel/chapter-planning-output.schema.json"
)

foreach ($schemaPath in $schemaPaths) {
    try {
        $schema = Get-Content -Raw $schemaPath | ConvertFrom-Json
    }
    catch {
        throw "JSON Schema validation failed for ${schemaPath}: $($_.Exception.Message)"
    }

    if ($schema.'$schema' -ne "https://json-schema.org/draft/2020-12/schema" -or
        [string]::IsNullOrWhiteSpace($schema.'$id') -or
        [string]::IsNullOrWhiteSpace($schema.title)) {
        throw "JSON Schema metadata validation failed for $schemaPath."
    }
}

$iteration05Operations = @(
    "listProjectChapterPlans",
    "mockGenerateProjectChapterPlans",
    "getChapterPlan",
    "updateChapterPlan",
    "deleteChapterPlan",
    "confirmProjectChapterPlans"
)
foreach ($operationId in $iteration05Operations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) {
        throw "Iteration 05 OpenAPI operation is missing: $operationId"
    }
}

$iteration05Paths = @(
    "/api/v1/projects/{projectId}/chapter-plans",
    "/api/v1/projects/{projectId}/chapter-plans/mock-generate",
    "/api/v1/chapter-plans/{chapterPlanId}",
    "/api/v1/projects/{projectId}/chapter-plans/confirm"
)
foreach ($path in $iteration05Paths) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) {
        throw "Iteration 05 OpenAPI path is missing: $path"
    }
}
foreach ($methodExpectation in @(
    @{ Path = "/api/v1/projects/{projectId}/chapter-plans"; Method = "get" },
    @{ Path = "/api/v1/projects/{projectId}/chapter-plans/mock-generate"; Method = "post" },
    @{ Path = "/api/v1/chapter-plans/{chapterPlanId}"; Method = "get" },
    @{ Path = "/api/v1/chapter-plans/{chapterPlanId}"; Method = "patch" },
    @{ Path = "/api/v1/chapter-plans/{chapterPlanId}"; Method = "delete" },
    @{ Path = "/api/v1/projects/{projectId}/chapter-plans/confirm"; Method = "post" }
)) {
    $pathPattern = "(?ms)^  " + [regex]::Escape($methodExpectation.Path) + ":\s*$.*?(?=^  /|^components:)"
    $pathBlock = [regex]::Match($openApiText, $pathPattern).Value
    if ($pathBlock -notmatch ("(?m)^    " + $methodExpectation.Method + ":\s*$")) {
        throw "Iteration 05 API method is missing: $($methodExpectation.Method.ToUpperInvariant()) $($methodExpectation.Path)"
    }
}
foreach ($errorExpectation in @(
    @{ OperationId = "listProjectChapterPlans"; Errors = @("400", "404") },
    @{ OperationId = "mockGenerateProjectChapterPlans"; Errors = @("400", "404", "409") },
    @{ OperationId = "getChapterPlan"; Errors = @("400", "404") },
    @{ OperationId = "updateChapterPlan"; Errors = @("400", "404", "409") },
    @{ OperationId = "deleteChapterPlan"; Errors = @("400", "404", "409") },
    @{ OperationId = "confirmProjectChapterPlans"; Errors = @("400", "404", "409") }
)) {
    $operationPattern = "(?ms)^\s*operationId:\s*" + [regex]::Escape($errorExpectation.OperationId) + "\s*$.*?(?=^\s*operationId:|^  /|^components:)"
    $operationBlock = [regex]::Match($openApiText, $operationPattern).Value
    foreach ($errorCode in $errorExpectation.Errors) {
        if ($operationBlock -notmatch ('(?m)^        "' + $errorCode + '": \{\$ref: "#/components/responses/')) {
            throw "Iteration 05 API error response is missing: $($errorExpectation.OperationId) $errorCode"
        }
    }
}
if ([regex]::Matches($openApiText, "(?m)^\s*operationId:\s*(listProjectChapterPlans|mockGenerateProjectChapterPlans|getChapterPlan|updateChapterPlan|deleteChapterPlan|confirmProjectChapterPlans)\s*$").Count -ne 6) {
    throw "Iteration 05 OpenAPI contains an unexpected or duplicate operationId."
}
foreach ($requiredFragment in @(
    'operationId: updateChapterPlan',
    'operationId: deleteChapterPlan',
    'operationId: confirmProjectChapterPlans',
    'expected_version',
    '"400": {$ref: "#/components/responses/BadRequest"}',
    '"404": {$ref: "#/components/responses/NotFound"}',
    '"409": {$ref: "#/components/responses/Conflict"}'
)) {
    if ($openApiText -notmatch [regex]::Escape($requiredFragment)) {
        throw "Iteration 05 required contract fragment is missing: $requiredFragment"
    }
}

$chapterPlanSchema = Get-Content -Raw "packages/contracts/content-packs/novel/chapter-plan.schema.json" | ConvertFrom-Json
$chapterPlanFields = @("chapter_no", "title", "summary", "storyline_refs_json", "material_refs_json", "foreshadowing_refs_json", "chapter_goal", "creation_notes")
if (($chapterPlanSchema.required -join ',') -ne ($chapterPlanFields -join ',')) {
    throw "Chapter-plan JSON Schema required fields do not match the frozen editable model."
}
if ($openApiText -notmatch [regex]::Escape('required: [id, project_id, chapter_no, title, summary, status, source, storyline_refs_json, material_refs_json, foreshadowing_refs_json, chapter_goal, creation_notes, confirmed_at, currentRevisionId, sourceCandidateId, sourceCandidateBatchId, sourceWorkflowRunId, version, created_at, updated_at]')) {
    throw "Chapter-plan OpenAPI response required fields do not match the frozen model."
}
foreach ($field in $chapterPlanFields) {
    if ($chapterPlanSchema.properties.PSObject.Properties.Name -notcontains $field -or
        $chapterPlanSchema.required -notcontains $field) {
        throw "Chapter-plan JSON Schema field/required mismatch: $field"
    }
    if ($openApiText -notmatch ("(?m)^        " + [regex]::Escape($field) + ":")) {
        throw "Chapter-plan OpenAPI field mismatch: $field"
    }
}
foreach ($nullableField in @("chapter_goal", "creation_notes")) {
    if ($chapterPlanSchema.properties.$nullableField.type -notcontains "null" -or
        $openApiText -notmatch ('(?m)^        ' + [regex]::Escape($nullableField) + ': \{type: \[string, "null"\]')) {
        throw "Chapter-plan nullable field mismatch: $nullableField"
    }
}
if ($chapterPlanSchema.properties.storyline_refs_json.items.properties.relation.enum -join ',' -ne 'primary,secondary') {
    throw "Chapter-plan JSON Schema relation enum mismatch."
}
if ($openApiText -notmatch 'enum: \[primary, secondary\]') {
    throw "Chapter-plan OpenAPI relation enum mismatch."
}
if ($openApiText -notmatch 'enum: \[pending_confirmation, confirmed\]' -or
    $openApiText -notmatch 'enum: \[mock_generated, candidate_adopted\]') {
    throw "Chapter-plan OpenAPI status/source enum mismatch."
}

$iteration06Operations = @("createContentItemForChapterPlan", "getContentItem", "saveContentItemDraft", "mockGenerateContentItem", "mockReviewContentItem", "listContentItemReviews", "getReview")
foreach ($operationId in $iteration06Operations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 06 OpenAPI operation is missing: $operationId" }
}
foreach ($path in @("/api/v1/chapter-plans/{chapterPlanId}/content", "/api/v1/content-items/{contentItemId}", "/api/v1/content-items/{contentItemId}/draft", "/api/v1/content-items/{contentItemId}/mock-generate", "/api/v1/content-items/{contentItemId}/reviews/mock", "/api/v1/content-items/{contentItemId}/reviews", "/api/v1/reviews/{reviewId}")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 06 OpenAPI path is missing: $path" }
}
foreach ($fragment in @("ContentItem", "ContentVersion", "ReviewReport", "WorkflowRunSummary", "expected_version", "Idempotency-Key", "content_version_already_reviewed", "created_at DESC, id DESC")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 06 required contract fragment is missing: $fragment" }
}
foreach ($schemaPath in $schemaPaths | Where-Object { $_ -match "(content-item|content-version|mock-generation|review-|workflow-run)" }) {
    $schema = Get-Content -Raw $schemaPath | ConvertFrom-Json
    if ($schema.additionalProperties -ne $false) { throw "Iteration 06 schema must set additionalProperties=false: $schemaPath" }
}

$iteration071AOperations = @("mockRewriteContentItem", "getWorkflowRunDetail")
foreach ($operationId in $iteration071AOperations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 07.1A OpenAPI operation is missing: $operationId" }
}
foreach ($path in @("/api/v1/content-items/{contentItemId}/rewrites/mock", "/api/v1/workflow-runs/{runId}")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 07.1A OpenAPI path is missing: $path" }
}
foreach ($fragment in @("MockRewriteRequest", "MockRewriteParameters", "MockRewriteResult", "WorkflowRunDetail", "content_mock_rewrite", "mock_rewrite", "idempotency_key_reused_with_different_payload")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 07.1A required contract fragment is missing: $fragment" }
}

$iteration071BOperations = @("listContentItemVersions", "getContentVersion")
foreach ($operationId in $iteration071BOperations) {
    if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 07.1B OpenAPI operation is missing: $operationId" }
}
foreach ($path in @("/api/v1/content-items/{contentItemId}/versions", "/api/v1/content-versions/{versionId}")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 07.1B OpenAPI path is missing: $path" }
}
foreach ($fragment in @("ContentVersionListEnvelope", "ContentVersionDetailEnvelope", "version_no DESC, id DESC", "ContentItem.current_version_id", "ContentVersionSourceSummary", "ContentVersionReviewSummary", "ContentVersionWorkflowRunSummary")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 07.1B required contract fragment is missing: $fragment" }
}

$iteration071COperations = @("listProjectWorks", "getProjectWork")
foreach ($operationId in $iteration071COperations) { if ($openApiText -notmatch ("(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$")) { throw "Iteration 07.1C OpenAPI operation is missing: $operationId" } }
foreach ($path in @("/api/v1/projects/{projectId}/works", "/api/v1/works/{workId}")) { if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 07.1C OpenAPI path is missing: $path" } }
foreach ($fragment in @("ProjectWorkReadModel", "ProjectWorkListEnvelope", "ProjectWorkDetailEnvelope", "Stable read-only alias of content_item.id", "chapter_plan.chapter_no ASC, content_item.id ASC")) { if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 07.1C required contract fragment is missing: $fragment" } }

$iteration08Operations = @("listMaterials", "listGlobalWorks", "listBuiltinWorkflows", "listWorkflowRuns", "listCapabilities", "listIntegrations")
foreach ($operationId in $iteration08Operations) {
    if ([regex]::Matches($openApiText, "(?m)^\s*operationId:\s*" + [regex]::Escape($operationId) + "\s*$").Count -ne 1) { throw "Iteration 08 OpenAPI operation must exist exactly once: $operationId" }
}
foreach ($path in @("/api/v1/materials", "/api/v1/works", "/api/v1/workflows/builtin", "/api/v1/workflow-runs", "/api/v1/capabilities", "/api/v1/integrations")) {
    if ($openApiText -notmatch ("(?m)^  " + [regex]::Escape($path) + ":\s*$")) { throw "Iteration 08 OpenAPI path is missing: $path" }
}
foreach ($fragment in @("GlobalWorkListEnvelope", "BuiltinWorkflowListEnvelope", "GlobalWorkflowRunListEnvelope", "CapabilityListEnvelope", "IntegrationListEnvelope", "GlobalScopeQuery", "current_version_id", "started_at DESC, id DESC", "not_available")) {
    if ($openApiText -notmatch [regex]::Escape($fragment)) { throw "Iteration 08 required contract fragment is missing: $fragment" }
}
$globalLiteSchema = Get-Content -Raw "packages/contracts/content-packs/novel/global-lite.schema.json" | ConvertFrom-Json
if ($globalLiteSchema.additionalProperties -ne $false -or $globalLiteSchema.'$defs'.builtin_workflow.properties.provider_key.const -ne "mock") { throw "Iteration 08 Global Lite JSON Schema drift." }

# CF-15 contract checks intentionally parse the OpenAPI and JSON Schema documents;
# they do not rely on text fragments because the contract uses discriminated unions.
@'
import json
from pathlib import Path
import re
import yaml

root = Path.cwd()
document = yaml.safe_load((root / "packages/contracts/openapi/openapi.yaml").read_text(encoding="utf-8"))
schemas = document["components"]["schemas"]
paths = document["paths"]

expected_operations = {
    "preflightChapterPlanRun", "createChapterPlanRun", "getProjectChapterPlanningSummary",
    "listChapterPlanCandidateBatches", "getChapterPlanCandidateBatch", "listChapterPlanCandidates",
    "getChapterPlanCandidate", "updateChapterPlanCandidate", "compareChapterPlanCandidate",
    "recompareChapterPlanCandidate", "adoptChapterPlanCandidate", "discardChapterPlanCandidate",
    "adoptChapterPlanCandidates", "abandonChapterPlanCandidateBatch", "listChapterPlanRevisions",
}
operation_ids = []
for path, path_item in paths.items():
    for method, operation in path_item.items():
        if method in {"get", "post", "patch", "put", "delete"}:
            operation_ids.append(operation.get("operationId"))
assert expected_operations <= set(operation_ids), "CF-15 operationId missing"
assert len(operation_ids) == len(set(operation_ids)), "OpenAPI operationId must be globally unique"

def resolve(ref):
    assert ref.startswith("#/"), f"external ref is not allowed in CF-15: {ref}"
    node = document
    for part in ref[2:].split("/"):
        node = node[part]
    return node

def check_refs(node):
    if isinstance(node, dict):
        if "$ref" in node:
            resolve(node["$ref"])
        for value in node.values():
            check_refs(value)
    elif isinstance(node, list):
        for value in node:
            check_refs(value)
check_refs(document)

request = schemas["ChapterPlanningPreflightRequest"]
assert request["additionalProperties"] is False and len(request["oneOf"]) == 3
for mode, target in (("full", "ChapterPlanningFullTarget"), ("append", "ChapterPlanningAppendTarget"), ("range", "ChapterPlanningRangeTarget")):
    assert any(branch["properties"]["generationMode"].get("const") == mode and branch["properties"]["target"]["$ref"].endswith(target) for branch in request["oneOf"])
assert schemas["ChapterPlanningRangeTarget"]["additionalProperties"] is False
assert schemas["ChapterPlanningFullTarget"]["properties"]["targetTotalChapters"]["maximum"] == 100
assert schemas["ChapterPlanningAppendTarget"]["properties"]["chapterCount"]["maximum"] == 100
def valid_preflight_input(payload):
    if set(payload) != {"generationMode", "target", "storylineSelection", "contextOptions", "additionalInstructions"}:
        return False
    mode, target = payload["generationMode"], payload["target"]
    expected_target = {"full": {"targetTotalChapters"}, "append": {"chapterCount"}, "range": {"startChapterNo", "endChapterNo"}}
    if mode not in expected_target or set(target) != expected_target[mode] or any(not isinstance(value, int) or value < 1 or value > 100 for value in target.values()):
        return False
    if mode == "range" and target["startChapterNo"] > target["endChapterNo"]:
        return False
    selection = payload["storylineSelection"]
    if selection.get("mode") == "auto_balanced":
        return set(selection) == {"mode"}
    if selection.get("mode") == "specified":
        return set(selection) == {"mode", "storylineIds"} and bool(selection["storylineIds"])
    return False
base_input = {"storylineSelection": {"mode": "auto_balanced"}, "contextOptions": {"includeProjectMaterials": True, "includeUnpaidForeshadowings": True, "includePriorChapterSummaries": True, "coreSettingsOnly": False}, "additionalInstructions": None}
for mode, target in (("full", {"targetTotalChapters": 100}), ("append", {"chapterCount": 1}), ("range", {"startChapterNo": 2, "endChapterNo": 3})):
    payload = dict(base_input, generationMode=mode, target=target)
    assert valid_preflight_input(payload)
for mode, target in (("full", {"chapterCount": 1}), ("append", {"targetTotalChapters": 1}), ("range", {"startChapterNo": 3, "endChapterNo": 2}), ("unknown", {"chapterCount": 1})):
    payload = dict(base_input, generationMode=mode, target=target)
    assert not valid_preflight_input(payload)

report = schemas["ChapterPlanningPreflightReport"]
assert len(report["oneOf"]) == 2
passed = schemas["ChapterPlanningPreflightPassed"]
blocked = schemas["ChapterPlanningPreflightBlocked"]
assert "preflightToken" in passed["required"] and "preflightToken" not in blocked["properties"]
assert passed["properties"]["blockers"]["maxItems"] == 0 and blocked["properties"]["blockers"]["minItems"] == 1

for name in ["ChapterPlanCandidateSnapshot", "ChapterPlanCandidate", "ChapterPlanCandidateBatch", "ChapterPlanCandidateDiff", "ChapterPlanRevision", "ChapterPlanningSummary", "ChapterPlanRevisionList"]:
    assert schemas[name]["additionalProperties"] is False, f"CF-15 schema must be strict: {name}"
for name in ["currentRevisionId", "sourceCandidateId", "sourceCandidateBatchId", "sourceWorkflowRunId"]:
    assert name in schemas["ChapterPlan"]["required"], f"ChapterPlan source field missing: {name}"

batch_parameters = paths["/api/v1/projects/{projectId}/chapter-plan-candidate-batches"]["get"]["parameters"]
assert {resolve(x["$ref"])["name"] for x in batch_parameters} >= {"status", "generationMode", "sourceWorkflowRunId", "createdAtFrom", "createdAtTo", "limit", "offset"}
bulk = schemas["BulkAdoptChapterPlanCandidateItem"]
assert {"candidateId", "expectedCandidateVersion", "expectedChapterPlanVersion"} <= set(bulk["required"])
candidate_list_parameters = paths["/api/v1/chapter-plan-candidate-batches/{batchId}/candidates"]["get"]["parameters"]
assert {resolve(x["$ref"])["name"] for x in candidate_list_parameters} == {"status", "diffType", "storylineId", "q", "limit", "offset"}
assert resolve(candidate_list_parameters[1]["$ref"])["schema"]["$ref"].endswith("ChapterPlanCandidateDiffType")
assert resolve(candidate_list_parameters[2]["$ref"])["schema"]["format"] == "uuid"
assert "not unbounded full-text search" in resolve(candidate_list_parameters[3]["$ref"])["description"]

def one_of_refs(schema_name):
    return [branch["$ref"].rsplit("/", 1)[-1] for branch in schemas[schema_name]["oneOf"]]

assert one_of_refs("AdoptChapterPlanCandidateResult") == ["AdoptChapterPlanCandidateAdoptedResult", "AdoptChapterPlanCandidateNoChangeResult"]
adopted = schemas["AdoptChapterPlanCandidateAdoptedResult"]
no_change = schemas["AdoptChapterPlanCandidateNoChangeResult"]
assert adopted["properties"]["outcome"]["const"] == "adopted" and adopted["properties"]["revision"]["$ref"].endswith("ChapterPlanRevision")
assert adopted["properties"]["candidate"]["$ref"].endswith("AdoptedChapterPlanCandidate")
assert no_change["properties"]["outcome"]["const"] == "no_change" and no_change["properties"]["revision"]["type"] == "null"
assert no_change["properties"]["candidate"]["$ref"].endswith("NonAdoptedChapterPlanCandidate")

assert one_of_refs("BulkAdoptChapterPlanCandidateResult") == ["BulkAdoptChapterPlanCandidateAdoptedResult", "BulkAdoptChapterPlanCandidateNoChangeResult", "BulkAdoptChapterPlanCandidateStaleResult", "BulkAdoptChapterPlanCandidateConflictResult", "BulkAdoptChapterPlanCandidateFailedResult"]
for branch_name, outcome in zip(one_of_refs("BulkAdoptChapterPlanCandidateResult"), ["adopted", "no_change", "stale", "conflict", "failed"]):
    branch = schemas[branch_name]
    assert branch["properties"]["outcome"]["const"] == outcome and "candidateId" in branch["required"]
assert schemas["BulkAdoptChapterPlanCandidateAdoptedResult"]["properties"]["revision"]["$ref"].endswith("ChapterPlanRevision")
assert schemas["BulkAdoptChapterPlanCandidateNoChangeResult"]["properties"]["revision"]["type"] == "null"
for branch_name in ["BulkAdoptChapterPlanCandidateStaleResult", "BulkAdoptChapterPlanCandidateConflictResult", "BulkAdoptChapterPlanCandidateFailedResult"]:
    branch = schemas[branch_name]
    assert "error" in branch["required"] and "revision" not in branch["properties"]

summary = schemas["ChapterPlanningInputSummary"]
assert summary["additionalProperties"] is False and set(summary["required"]) == {"generationMode", "target", "storylineSelection", "contextOptions"}
target_summary = schemas["ChapterPlanningNormalizedTargetSummary"]
assert target_summary["additionalProperties"] is False and set(target_summary["required"]) == {"startChapterNo", "endChapterNo", "requestedChapterCount"}
assert all(target_summary["properties"][field]["minimum"] == 1 and target_summary["properties"][field]["maximum"] == 100 for field in target_summary["required"])

def summary_valid(value):
    required = set(target_summary["required"])
    target = value.get("target", {})
    return set(value) == set(summary["required"]) and set(target) == required and all(isinstance(target[field], int) and 1 <= target[field] <= 100 for field in required) and target["endChapterNo"] - target["startChapterNo"] + 1 == target["requestedChapterCount"]
valid_summary = {"generationMode": "range", "target": {"startChapterNo": 2, "endChapterNo": 3, "requestedChapterCount": 2}, "storylineSelection": {"mode": "auto_balanced"}, "contextOptions": base_input["contextOptions"]}
assert summary_valid(valid_summary)
for missing in ["startChapterNo", "endChapterNo", "requestedChapterCount"]:
    invalid_summary = json.loads(json.dumps(valid_summary)); del invalid_summary["target"][missing]; assert not summary_valid(invalid_summary)

for path, method, request_schema, expected_version in [
    ("/api/v1/projects/{projectId}/chapter-plan-runs", "post", "CreateChapterPlanRunRequest", None),
    ("/api/v1/chapter-plan-candidates/{candidateId}", "patch", "UpdateChapterPlanCandidateRequest", "expectedCandidateVersion"),
    ("/api/v1/chapter-plan-candidates/{candidateId}/recompare", "post", "RecompareChapterPlanCandidateRequest", "expectedCandidateVersion"),
    ("/api/v1/chapter-plan-candidates/{candidateId}/adopt", "post", "AdoptChapterPlanCandidateRequest", "expectedCandidateVersion"),
    ("/api/v1/chapter-plan-candidates/{candidateId}/discard", "post", "DiscardChapterPlanCandidateRequest", "expectedCandidateVersion"),
    ("/api/v1/chapter-plan-candidate-batches/{batchId}/adoptions", "post", "BulkAdoptChapterPlanCandidatesRequest", "expectedBatchVersion"),
    ("/api/v1/chapter-plan-candidate-batches/{batchId}/abandon", "post", "AbandonChapterPlanCandidateBatchRequest", "expectedBatchVersion"),
]:
    operation = paths[path][method]
    assert any(resolve(x["$ref"])["name"] == "Idempotency-Key" and resolve(x["$ref"])["required"] for x in operation.get("parameters", []) + paths[path].get("parameters", [])), f"Idempotency-Key missing: {path}"
    assert operation["requestBody"]["content"]["application/json"]["schema"]["$ref"].endswith(request_schema)
    if expected_version:
        assert expected_version in schemas[request_schema]["required"], f"expected version missing: {request_schema}"

expected_error_codes = {"workflow_not_configured", "preflight_token_invalid", "preflight_token_expired", "preflight_input_changed", "active_run_conflict", "invalid_candidate_state", "stale_candidate", "version_conflict", "batch_already_finalized", "run_already_consumed", "chapter_no_conflict", "revision_sequence_conflict", "output_validation_failed", "result_consumption_failed", "idempotency_key_reused_with_different_payload"}
assert expected_error_codes == set(schemas["ChapterPlanningErrorCode"]["enum"])
assert {"project_binding_missing", "execution_integration_unavailable", "active_run_conflict", "storyline_reference_invalid", "generation_input_invalid"} == set(schemas["ChapterPlanningBlockerCode"]["enum"])

details = schemas["ChapterPlanningErrorDetails"]
assert details["additionalProperties"] is False and {"retryAction", "safeReason"} <= set(details["required"])
def response_schema(path, method, status):
    return paths[path][method]["responses"][str(status)]["content"]["application/json"]["schema"]["$ref"].rsplit("/", 1)[-1]
def codes_for_error_envelope(envelope_name):
    envelope = schemas[envelope_name]
    assert envelope["allOf"][0]["$ref"].endswith("ErrorEnvelope"), f"CF-15 response must refine ErrorEnvelope: {envelope_name}"
    error_name = envelope["allOf"][1]["properties"]["error"]["$ref"].rsplit("/", 1)[-1]
    error = schemas[error_name]
    code = error["allOf"][1]["properties"]["code"]
    return {code["const"]} if "const" in code else set(code["enum"])
error_response_expectations = {
    ("/api/v1/projects/{projectId}/chapter-plan-runs/preflight", "post", 409): ({"active_run_conflict"}, "ChapterPlanningPreflightConflictErrorEnvelope"),
    ("/api/v1/projects/{projectId}/chapter-plan-runs/preflight", "post", 422): ({"workflow_not_configured"}, "ChapterPlanningWorkflowNotConfiguredErrorEnvelope"),
    ("/api/v1/projects/{projectId}/chapter-plan-runs", "post", 409): ({"preflight_input_changed", "active_run_conflict", "run_already_consumed", "idempotency_key_reused_with_different_payload"}, "ChapterPlanningCreateRunConflictErrorEnvelope"),
    ("/api/v1/projects/{projectId}/chapter-plan-runs", "post", 422): ({"workflow_not_configured", "preflight_token_invalid", "preflight_token_expired"}, "ChapterPlanningCreateRunPreconditionErrorEnvelope"),
    ("/api/v1/chapter-plan-candidates/{candidateId}/adopt", "post", 409): ({"invalid_candidate_state", "stale_candidate", "version_conflict", "chapter_no_conflict", "revision_sequence_conflict", "idempotency_key_reused_with_different_payload"}, "ChapterPlanningSingleAdoptConflictErrorEnvelope"),
    ("/api/v1/chapter-plan-candidate-batches/{batchId}/adoptions", "post", 409): ({"batch_already_finalized", "version_conflict", "idempotency_key_reused_with_different_payload"}, "ChapterPlanningBulkAdoptConflictErrorEnvelope"),
    ("/api/v1/chapter-plan-candidate-batches/{batchId}/abandon", "post", 409): ({"invalid_candidate_state", "batch_already_finalized", "version_conflict", "idempotency_key_reused_with_different_payload"}, "ChapterPlanningBatchConflictErrorEnvelope"),
    ("/api/v1/projects/{projectId}/chapter-planning-summary", "get", 422): ({"output_validation_failed"}, "ChapterPlanningOutputValidationErrorEnvelope"),
    ("/api/v1/projects/{projectId}/chapter-planning-summary", "get", 500): ({"result_consumption_failed"}, "ChapterPlanningResultConsumptionErrorEnvelope"),
}
all_bound_error_codes = set()
for (path, method, status), (expected_codes, expected_envelope) in error_response_expectations.items():
    envelope_name = response_schema(path, method, status)
    assert envelope_name == expected_envelope and codes_for_error_envelope(envelope_name) == expected_codes
    all_bound_error_codes |= expected_codes
assert all_bound_error_codes == expected_error_codes
for example_name in ["ChapterPlanningPreflightTokenExpiredError", "ChapterPlanningActiveRunConflictError", "ChapterPlanningStaleCandidateError", "ChapterPlanningBatchAlreadyFinalizedError", "ChapterPlanningOutputValidationFailedError"]:
    assert example_name in document["components"]["examples"], f"CF-15 error example missing: {example_name}"

iteration15 = root / "docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning"
manifest = json.loads((iteration15 / "ui-manifest.json").read_text(encoding="utf-8"))
traceability = (iteration15 / "ui-contract-traceability.md").read_text(encoding="utf-8")
trace_frames = set(re.findall(r"^\| (P15_[A-Z0-9_]+) \|", traceability, re.MULTILINE))
manifest_frames = {frame["frameId"] for frame in manifest["frames"]}
assert manifest["frameCount"] == 21 and len(manifest_frames) == 21 and trace_frames == manifest_frames, "Iteration 15 UI traceability must be 21/21"
assert len({operation for operation in expected_operations if operation not in {"listChapterPlanRevisions"}}) < 21, "Frames are UI states/components, not routes"

failed_atomic_file = "docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/ui-contract-traceability.md"
openapi_file = "packages/contracts/openapi/openapi.yaml"
failed_atomic_lines = [line for line in traceability.splitlines() if line.startswith("| P15_C1_FAILED_ATOMIC |")]
failed_atomic_line = failed_atomic_lines[0] if len(failed_atomic_lines) == 1 else "\\n".join(failed_atomic_lines)
failed_atomic_failures = []
def failed_atomic_check(item, passed, file, expected, actual):
    if not passed:
        failed_atomic_failures.append(
            f"[FAIL] {item}\\n"
            f"  file: {file}\\n"
            f"  expected: {expected}\\n"
            f"  actual: {actual}"
        )

failed_atomic_check(
    "P15_C1_FAILED_ATOMIC entry exists exactly once",
    len(failed_atomic_lines) == 1,
    failed_atomic_file,
    "exactly one P15_C1_FAILED_ATOMIC traceability row",
    f"{len(failed_atomic_lines)} matching rows",
)
failed_atomic_check(
    "P15_C1_FAILED_ATOMIC names both ownership APIs",
    "getWorkflowRunDetail" in failed_atomic_line and "listWorkflowRunEvents" in failed_atomic_line and "getProjectChapterPlanningSummary" in failed_atomic_line,
    failed_atomic_file,
    "Runtime getWorkflowRunDetail/listWorkflowRunEvents and getProjectChapterPlanningSummary",
    failed_atomic_line or "missing row",
)
failed_atomic_check(
    "P15_C1_FAILED_ATOMIC assigns failed to Runtime status",
    "`status=failed`" in failed_atomic_line and "`failed` is Runtime status" in failed_atomic_line,
    failed_atomic_file,
    "failed explicitly belongs to Runtime status",
    failed_atomic_line or "missing row",
)
failed_atomic_check(
    "P15_C1_FAILED_ATOMIC assigns output_validation_failed to Summary HTTP 422",
    "`output_validation_failed` is Summary HTTP 422 ErrorEnvelope only" in failed_atomic_line,
    failed_atomic_file,
    "output_validation_failed only as getProjectChapterPlanningSummary HTTP 422",
    failed_atomic_line or "missing row",
)
failed_atomic_check(
    "P15_C1_FAILED_ATOMIC assigns result_consumption_failed to Summary HTTP 500",
    "`result_consumption_failed` is Summary HTTP 500 ErrorEnvelope only" in failed_atomic_line,
    failed_atomic_file,
    "result_consumption_failed only as getProjectChapterPlanningSummary HTTP 500",
    failed_atomic_line or "missing row",
)
failed_atomic_check(
    "P15_C1_FAILED_ATOMIC rejects Runtime HTTP ownership for Summary errors",
    "neither is a Runtime HTTP error" in failed_atomic_line and not re.search(r"Runtime(?: detail/events)? HTTP (?:422|500).*(?:output_validation_failed|result_consumption_failed)", failed_atomic_line),
    failed_atomic_file,
    "output_validation_failed and result_consumption_failed are not Runtime HTTP errors",
    failed_atomic_line or "missing row",
)
failed_atomic_check(
    "P15_C1_FAILED_ATOMIC documents Runtime then Summary handling",
    "Runtime detail/events first return" in failed_atomic_line and "The page then refreshes Summary" in failed_atomic_line,
    failed_atomic_file,
    "Runtime failed state is handled before the Summary refresh/error handling",
    failed_atomic_line or "missing row",
)

summary_path = "/api/v1/projects/{projectId}/chapter-planning-summary"
summary_operation = paths.get(summary_path, {}).get("get", {})
summary_response_actual = {}
for status in ("422", "500"):
    schema = summary_operation.get("responses", {}).get(status, {}).get("content", {}).get("application/json", {}).get("schema", {})
    envelope_name = schema.get("$ref", "").rsplit("/", 1)[-1]
    try:
        codes = sorted(codes_for_error_envelope(envelope_name)) if envelope_name else []
    except (KeyError, IndexError, TypeError, AssertionError):
        codes = []
    summary_response_actual[status] = {"envelope": envelope_name or "missing", "codes": codes}
failed_atomic_check(
    "OpenAPI Summary operation ownership",
    summary_operation.get("operationId") == "getProjectChapterPlanningSummary",
    openapi_file,
    "GET chapter-planning-summary operationId=getProjectChapterPlanningSummary",
    summary_operation.get("operationId", "missing"),
)
failed_atomic_check(
    "OpenAPI Summary HTTP 422 ownership",
    summary_response_actual["422"] == {"envelope": "ChapterPlanningOutputValidationErrorEnvelope", "codes": ["output_validation_failed"]},
    openapi_file,
    "HTTP 422 ChapterPlanningOutputValidationErrorEnvelope with output_validation_failed",
    summary_response_actual["422"],
)
failed_atomic_check(
    "OpenAPI Summary HTTP 500 ownership",
    summary_response_actual["500"] == {"envelope": "ChapterPlanningResultConsumptionErrorEnvelope", "codes": ["result_consumption_failed"]},
    openapi_file,
    "HTTP 500 ChapterPlanningResultConsumptionErrorEnvelope with result_consumption_failed",
    summary_response_actual["500"],
)
failed_atomic_check(
    "Iteration 15 UI Frame traceability remains 21/21",
    manifest["frameCount"] == 21 and len(manifest_frames) == 21 and trace_frames == manifest_frames,
    failed_atomic_file,
    "21 manifest frames and the same 21 traceability frame IDs",
    f"manifest frameCount={manifest['frameCount']}, manifest IDs={len(manifest_frames)}, trace IDs={len(trace_frames)}",
)
if failed_atomic_failures:
    raise AssertionError("CF-15 Failed Atomic ownership validation failed:\\n" + "\\n".join(failed_atomic_failures))
print("[PASS] CF-15 Failed Atomic UI/API ownership validation completed.")

blocked_trace_line = next(line for line in traceability.splitlines() if line.startswith("| P15_C3_PREFLIGHT_BLOCKED |"))
assert "preflight_blocked" not in blocked_trace_line and "UI state=`blocked`" in blocked_trace_line and "ChapterPlanningBlockerCode" in blocked_trace_line
assert "active_run_conflict" in blocked_trace_line and "ChapterPlanningBlockerCode" in blocked_trace_line

output_schema = json.loads((root / "packages/contracts/content-packs/novel/chapter-planning-output.schema.json").read_text(encoding="utf-8"))
assert output_schema["$id"] == "https://ai-content-factory.local/schemas/content-packs/novel/chapter-planning-output.schema.json"
assert output_schema["additionalProperties"] is False and output_schema["$defs"]["candidate"]["additionalProperties"] is False
valid = json.loads((root / "packages/contracts/content-packs/novel/fixtures/chapter-planning-output.valid.json").read_text(encoding="utf-8"))
def semantic_valid(payload):
    target = payload["target"]
    candidates = payload["candidates"]
    chapter_numbers = [candidate["chapterNo"] for candidate in candidates]
    if len(chapter_numbers) != len(set(chapter_numbers)) or len(candidates) != target["requestedChapterCount"]:
        return False
    if target["startChapterNo"] > target["endChapterNo"] or any(no < target["startChapterNo"] or no > target["endChapterNo"] for no in chapter_numbers):
        return False
    return all(ref["projectId"] == payload["projectId"] for candidate in candidates for key in ("storylineRefs", "materialRefs", "foreshadowingRefs") for ref in candidate[key])
assert semantic_valid(valid)
for fixture_name in ("chapter-planning-output.duplicate-chapter.json", "chapter-planning-output.out-of-target.json", "chapter-planning-output.cross-project-reference.json"):
    invalid = json.loads((root / "packages/contracts/content-packs/novel/fixtures" / fixture_name).read_text(encoding="utf-8"))
    assert not semantic_valid(invalid), f"CF-15 semantic fixture unexpectedly passed: {fixture_name}"
print("[PASS] CF-15 structured OpenAPI and normalized-output semantic validation completed.")
'@ | python -
if ($LASTEXITCODE -ne 0) { throw "CF-15 structured contract validation failed." }

npx.cmd --yes ajv-cli@5.0.0 --spec=draft2020 --validate-formats=false -s packages/contracts/content-packs/novel/chapter-planning-output.schema.json -d packages/contracts/content-packs/novel/fixtures/chapter-planning-output.valid.json
if ($LASTEXITCODE -ne 0) { throw "CF-15 normalized output valid fixture failed." }
$savedErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$topUnknownOutput = (& npx.cmd --yes ajv-cli@5.0.0 --spec=draft2020 --validate-formats=false -s packages/contracts/content-packs/novel/chapter-planning-output.schema.json -d packages/contracts/content-packs/novel/fixtures/chapter-planning-output.unknown-field.json 2>&1 | Out-String)
 $topUnknownExitCode = $LASTEXITCODE
$candidateUnknownOutput = (& npx.cmd --yes ajv-cli@5.0.0 --spec=draft2020 --validate-formats=false -s packages/contracts/content-packs/novel/chapter-planning-output.schema.json -d packages/contracts/content-packs/novel/fixtures/chapter-planning-output.candidate-unknown-field.json 2>&1 | Out-String)
$candidateUnknownExitCode = $LASTEXITCODE
$ErrorActionPreference = $savedErrorActionPreference
if ($topUnknownExitCode -eq 0 -or $topUnknownOutput -notmatch "additionalProperty: 'unexpected'" -or $topUnknownOutput -match "minItems|required") { throw "CF-15 top-level unknown-field fixture did not fail only for unexpected." }
if ($candidateUnknownExitCode -eq 0 -or $candidateUnknownOutput -notmatch "additionalProperty: 'unexpectedCandidate'" -or $candidateUnknownOutput -match "minItems|required") { throw "CF-15 candidate unknown-field fixture did not fail only for unexpectedCandidate." }

Write-Host "[PASS] OpenAPI and Novel JSON Schema validation completed." -ForegroundColor Green

