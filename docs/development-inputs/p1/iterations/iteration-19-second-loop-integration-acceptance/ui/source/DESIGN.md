# ACF Enterprise Desktop

## Product shell
- Desktop enterprise SaaS application.
- Preserve the existing ACF AppShell: fixed white left sidebar, compact white top bar, pale gray application canvas.
- Sidebar width 240px; top bar height 64px; content padding 24px.
- Do not alter navigation information architecture.

## Visual language
- Primary accent: #5146E5.
- Canvas: #F6F7FB.
- Surface: #FFFFFF.
- Border: #E7E9F0.
- Main text: #1A1C21; secondary text: #6B7280.
- Success #16A36A, warning #D98C10, error #D14343.
- No gradients, no decorative illustrations, no heavy shadows.
- Compact enterprise tables, forms, drawers and dialogs.
- Standard control height 32px; large controls 40px; radius 6px.

## Interaction rules
- Provider, Connection and Workflow Configuration separate enabled state from runtime eligibility.
- Editing verified key fields preserves enabled state and bindings, but sets configuration changed / not executable until re-verified.
- LLM strategy belongs only to Workflow Configuration; project binding cannot override Provider or model.
- WorkflowRun detail remains an independent page at /workflow-runs/[runId].
- Four business stages share the same binding and runtime-state component structures.
- Never display secrets, credentials, authorization headers, raw upstream responses or stack traces.