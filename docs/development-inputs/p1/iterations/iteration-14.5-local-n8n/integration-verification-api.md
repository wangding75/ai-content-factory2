# Integration verification API

Verify is the explicit lifecycle operation that proves a saved Connection or
Workflow Configuration can be reached. Create and update remain local-only and
cannot write `enabled` or `integrationStatus`; Disable is the explicit local
operation for turning off an already saved resource.

## State transitions

New resources start as `not_connected` and disabled. A successful Connection
verify performs the n8n health probe and changes it to `connected` and enabled.
A failed Connection verify persists `not_connected` and disabled. A Workflow
Configuration verify first requires its Connection to be connected and enabled;
its successful probe changes only the Workflow Configuration to `connected` and
enabled, while a failed probe changes only that configuration to
`not_connected` and disabled. Disable only changes the target `enabled` flag
and is idempotent at the current version.

## Formal endpoints

- `POST /api/v1/workflow-connections/{connectionId}/verify`
- `POST /api/v1/workflow-connections/{connectionId}/disable`
- `POST /api/v1/workflow-configurations/{workflowId}/verify`
- `POST /api/v1/workflow-configurations/{workflowId}/disable`

Each action requires `Idempotency-Key` and a JSON body containing
`expectedVersion`. Success returns the updated resource in the standard
envelope. A safe `422 verification_failed` response contains no upstream body,
credential, cookie, password, token, or internal network detail.

## Local n8n probe

Workflow verification posts to the production webhook using a dedicated payload:

```json
{"probeType":"acf_workflow_verification","stage":"chapter_planning","contractVersion":"v1","requestId":"unique-id"}
```

The local workflow returns `verified`, `stage`, `contractVersion`, and the same
`requestId`. This branch must not invoke an LLM, access the ACF database, or
create an ACF WorkflowRun, batch, or candidate. Normal chapter-planning payloads
continue on their existing deterministic path; invalid payloads return HTTP 400.

## Local verification sequence

Start the API, import and activate the workflow with
`scripts/setup-local-n8n-workflow.ps1`, create isolated resources through the
formal API, then verify the Connection and Workflow Configuration. Disable each
resource and verify again with its current version to confirm the state can be
restored. Do not bind a project, call Preflight, or create a run during this
check.

Iteration 14.5-03C may later add project binding and Preflight validation after
these lifecycle operations are available; it is outside this task.
