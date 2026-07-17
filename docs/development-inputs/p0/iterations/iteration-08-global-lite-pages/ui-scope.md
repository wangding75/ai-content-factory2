# UI 范围

- `E1_GLOBAL_MATERIALS`｜全局素材
- `E2_GLOBAL_WORKS`｜全局作品
- `E3_WORKFLOWS`｜流程中心
- `E4_SETTINGS`｜全局设置

实际 UI 文件位于 `ui/frames/`。

## Frozen top-level routes and Lite action boundary

| Frame | Route | Read data | Allowed action and target | Excluded |
|---|---|---|---|---|
| E1_GLOBAL_MATERIALS | `/materials` | `listMaterials`, then `getMaterial` for selected reference details | Filter/page the list; open an existing `/projects/{projectId}/materials/{materialId}` only when that reference supplies both identifiers | Create, edit, bind or unbind material |
| E2_GLOBAL_WORKS | `/works` | `listGlobalWorks` over the existing ProjectWork read model | Filter/page; open existing `/projects/{projectId}/works` with `projectId` | Work persistence/lifecycle and a global work-detail route |
| E3_WORKFLOWS | `/workflows` | `listBuiltinWorkflows`, `listGlobalWorkflowRuns`; `getWorkflowRun` only for a selected run | Filter/page; open `/projects/{projectId}/works` only when the run returns a non-null project id | Editor, orchestration, nodes, drag/drop and execution |
| E4_SETTINGS | `/settings` | `listCapabilities`, `listIntegrations` | Read status; navigate to `/workflows` for built-in workflow capability | Any configuration, API key, provider, authority, billing, team or publishing write action |

All routes above are formal implementation targets. They supersede the current placeholder web-navigation targets. Every data request has loading, success, empty and safe-error states; filters and pagination are client request state, and no Lite action may use a blank target, `href="#"`, or a modal-only placeholder.
