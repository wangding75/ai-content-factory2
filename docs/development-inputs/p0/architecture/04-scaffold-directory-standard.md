
# AI Content Factory 2.0｜脚手架与目录规范

## 1. 总体原则

技术骨架与 1.0 相同，2.0 在模块、契约和扩展点上增加以下能力：

- Content Pack。
- Workflow Provider。
- 资产与项目用途分离。
- 内容版本与审核结果分离。
- `.ai-dev` 迭代状态管理。
- 按闭环迭代组织任务和验收。

目录不是建议清单，而是默认工程规范。偏离时必须提交 ADR。

## 2. Monorepo 根目录

```text
ai-content-factory/
├── apps/
│   ├── api/
│   └── web/
├── packages/
│   ├── contracts/
│   ├── shared-types/
│   ├── eslint-config/
│   └── tsconfig/
├── docs/
│   ├── product/
│   ├── architecture/
│   ├── api/
│   ├── testing/
│   ├── decisions/
│   ├── iterations/
│   └── prototypes/
├── deployments/
│   ├── docker/
│   ├── compose/
│   └── k8s/
├── scripts/
├── tasks/
├── tests/
│   ├── e2e/
│   ├── contract/
│   └── fixtures/
├── .ai-dev/
├── .github/
│   └── workflows/
├── Makefile
├── docker-compose.yml
├── package.json
├── pnpm-workspace.yaml
├── go.work
├── .env.example
├── .editorconfig
├── .gitattributes
├── .gitignore
└── README.md
```

## 3. Go API 目录

```text
apps/api/
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── platform/
│   │   ├── config/
│   │   ├── database/
│   │   ├── httpserver/
│   │   ├── logging/
│   │   ├── telemetry/
│   │   ├── validation/
│   │   ├── clock/
│   │   └── idgen/
│   ├── project/
│   ├── material/
│   ├── narrative/
│   ├── chapterplan/
│   ├── content/
│   ├── review/
│   ├── workflow/
│   ├── works/
│   ├── capability/
│   └── audit/
├── plugins/
│   ├── contentpacks/
│   │   └── novel/
│   └── workflowproviders/
│       └── mock/
├── migrations/
│   ├── 000001_init.up.sql
│   ├── 000001_init.down.sql
│   └── ...
├── test/
│   ├── integration/
│   ├── fixtures/
│   └── testutil/
├── go.mod
├── go.sum
└── README.md
```

## 4. Go 业务模块模板

以 `material` 为例：

```text
internal/material/
├── domain/
│   ├── material.go
│   ├── project_material_usage.go
│   ├── material_type.go
│   ├── repository.go
│   ├── service.go
│   ├── events.go
│   └── errors.go
├── application/
│   ├── commands/
│   │   ├── create_material.go
│   │   ├── create_and_bind_material.go
│   │   ├── bind_material.go
│   │   ├── update_material.go
│   │   ├── update_usage.go
│   │   └── unbind_material.go
│   ├── queries/
│   │   ├── get_material.go
│   │   ├── list_global_materials.go
│   │   └── list_project_materials.go
│   ├── dto/
│   │   ├── request.go
│   │   └── response.go
│   └── ports/
│       └── audit.go
├── interfaces/
│   └── http/
│       ├── handler.go
│       ├── request.go
│       ├── response.go
│       └── routes.go
└── infrastructure/
    └── postgres/
        ├── repository.go
        ├── mapper.go
        └── queries.sql
```

### 文件职责

| 文件 | 职责 |
|---|---|
| domain/entity | 业务状态与行为 |
| domain/repository | 仓储接口 |
| application/commands | 写用例 |
| application/queries | 读用例 |
| application/dto | 用例输入输出 |
| interfaces/http | HTTP 适配 |
| infrastructure/postgres | PostgreSQL 实现 |
| mapper | DB Row、Domain、DTO 映射 |

### 禁止

- `handler.go` 直接写 SQL。
- Domain import PostgreSQL、HTTP 或 Redis 包。
- Repository 返回 HTTP DTO。
- Application 直接依赖具体 Postgres Repository。
- 多个模块共享数据库 Row struct。

## 5. Web 目录

```text
apps/web/
├── src/
│   ├── app/
│   │   ├── (global)/
│   │   │   ├── page.tsx
│   │   │   ├── projects/
│   │   │   ├── materials/
│   │   │   ├── works/
│   │   │   ├── workflows/
│   │   │   └── settings/
│   │   └── projects/
│   │       └── [projectId]/
│   │           ├── page.tsx
│   │           ├── planning/
│   │           ├── materials/
│   │           ├── storylines/
│   │           ├── chapters/
│   │           ├── reviews/
│   │           └── works/
│   ├── widgets/
│   │   ├── app-shell/
│   │   ├── project-shell/
│   │   └── page-header/
│   ├── features/
│   │   ├── create-project/
│   │   ├── manage-material/
│   │   ├── manage-storyline/
│   │   ├── generate-chapter-plan/
│   │   ├── edit-content/
│   │   ├── review-content/
│   │   └── create-rewrite/
│   ├── entities/
│   │   ├── project/
│   │   ├── material/
│   │   ├── storyline/
│   │   ├── chapter-plan/
│   │   ├── content-item/
│   │   ├── review/
│   │   └── workflow-run/
│   ├── shared/
│   │   ├── api/
│   │   ├── config/
│   │   ├── lib/
│   │   ├── hooks/
│   │   ├── ui/
│   │   ├── styles/
│   │   ├── types/
│   │   └── test/
│   └── generated/
│       └── api/
├── public/
├── tests/
│   ├── unit/
│   └── integration/
├── next.config.ts
├── tsconfig.json
├── package.json
└── README.md
```

## 6. Web Feature 模板

```text
features/create-project/
├── api/
│   ├── create-project.ts
│   └── keys.ts
├── model/
│   ├── schema.ts
│   ├── types.ts
│   └── use-create-project.ts
├── ui/
│   ├── create-project-form.tsx
│   └── create-project-dialog.tsx
├── test/
│   └── create-project.test.tsx
└── index.ts
```

规则：

- Feature 对外只通过 `index.ts` 暴露。
- `app` 负责组装，不承载业务规则。
- `shared/ui` 不引用业务实体。
- DTO 类型来自 `generated/api` 或 `packages/shared-types`。
- Query Key 在 feature/entity 的 `api/keys.ts` 统一定义。
- 表单校验 Schema 与 API 契约字段一致。

## 7. Contracts 目录

```text
packages/contracts/
├── openapi/
│   ├── openapi.yaml
│   ├── paths/
│   │   ├── projects.yaml
│   │   ├── materials.yaml
│   │   ├── storylines.yaml
│   │   ├── chapter-plans.yaml
│   │   ├── contents.yaml
│   │   ├── reviews.yaml
│   │   ├── workflows.yaml
│   │   └── works.yaml
│   └── schemas/
│       ├── common/
│       ├── project/
│       ├── material/
│       ├── narrative/
│       ├── chapter-plan/
│       ├── content/
│       ├── review/
│       └── workflow/
├── content-packs/
│   └── novel/
│       ├── project.schema.json
│       ├── planning.schema.json
│       ├── material-usage.schema.json
│       └── chapter-plan.schema.json
├── workflow-providers/
│   └── mock/
│       ├── execute-request.schema.json
│       └── execute-result.schema.json
└── README.md
```

契约优先顺序：

```text
OpenAPI / JSON Schema
→ 生成类型
→ 后端 Handler / DTO
→ 前端 API Client
```

禁止先改实现再补契约。

## 8. 数据库迁移规范

### 命名

```text
000001_init_core
000002_add_projects
000003_add_materials
000004_add_narrative
000005_add_chapter_plans
000006_add_content_reviews
000007_add_workflow_runs
```

### 规则

- 每个 migration 同时提供 up/down。
- 一个 migration 只处理一个清晰目的。
- 禁止修改已经进入共享分支的历史 migration。
- 新增非空字段必须提供兼容迁移路径。
- 唯一约束、外键和 CHECK 不能只写在 Go 代码中。
- Migration 必须在空库和上一版本数据库上验证。

## 9. 测试目录

```text
tests/
├── contract/
│   ├── openapi_test.*
│   └── schema_test.*
├── e2e/
│   ├── fixtures/
│   ├── pages/
│   ├── specs/
│   │   ├── project-creation.spec.ts
│   │   ├── material-lifecycle.spec.ts
│   │   ├── narrative.spec.ts
│   │   ├── chapter-planning.spec.ts
│   │   ├── editor-review.spec.ts
│   │   ├── rewrite-versions.spec.ts
│   │   └── p0-full-chain.spec.ts
│   └── playwright.config.ts
└── fixtures/
    ├── projects/
    ├── materials/
    └── workflows/
```

E2E Page Object 使用 Frame ID：

```ts
export class ChapterPlanningPage {
  readonly frameId = "C1_CHAPTER_PLANNING";
}
```

## 10. 文档目录

```text
docs/
├── product/
│   ├── business-architecture.md
│   ├── product-architecture.md
│   ├── p0-scope.md
│   └── p0-link-map.md
├── architecture/
│   ├── technical-architecture.md
│   ├── data-model.md
│   ├── status-machines.md
│   └── scaffold-directory-standard.md
├── api/
│   ├── api-catalog.md
│   └── error-codes.md
├── testing/
│   ├── acceptance-principles.md
│   └── traceability-matrix.csv
├── decisions/
│   └── ADR-xxxx-title.md
├── iterations/
│   └── iteration-xx/
└── prototypes/
    └── p0-frames/
```

## 11. Tasks 目录

```text
tasks/
├── iteration-00-contract-freeze.md
├── iteration-01-scaffold-infrastructure.md
├── iteration-02-project-creation.md
├── iteration-03-planning-materials.md
├── iteration-04-storylines-foreshadowing.md
├── iteration-05-chapter-planning.md
├── iteration-06-editor-review.md
├── iteration-07-rewrite-works.md
├── iteration-08-global-lite-pages.md
└── iteration-09-p0-e2e-acceptance.md
```

每个任务文件必须包含：

```text
目标
前置依赖
闭环链路
UI
API
数据模型
状态变化
开发任务
自动化测试
验收用例
明确排除
完成定义
```

## 12. `.ai-dev` 规范

```text
.ai-dev/
├── state.json
├── iterations/
│   ├── 00.json
│   ├── 01.json
│   └── ...
├── reports/
│   ├── iteration-00-plan.md
│   ├── iteration-00-result.md
│   └── ...
└── templates/
    ├── execution-plan.md
    └── completion-report.md
```

`state.json` 示例：

```json
{
  "project": "ai-content-factory-2.0",
  "current_iteration": 0,
  "status": "planned",
  "next_iteration": 1,
  "contract_version": "p0-v1",
  "ui_baseline": "p0-frozen",
  "updated_at": "2026-07-10T00:00:00Z"
}
```

状态：

```text
planned
approved
in_progress
verifying
completed
blocked
```

## 13. Scripts 与 Makefile

```text
scripts/
├── bootstrap.sh
├── dev.sh
├── test.sh
├── verify-contracts.sh
├── verify-migrations.sh
├── verify-iteration.sh
├── finish-iteration.sh
├── seed-p0.sh
└── clean.sh
```

Makefile 建议：

```makefile
bootstrap:
dev:
up:
down:
migrate:
seed:
test:
test-api:
test-web:
test-e2e:
check-contracts:
check-docs:
verify:
verify-iteration:
finish-iteration:
```

## 14. 环境变量

`.env.example`：

```text
APP_ENV=development
API_PORT=8080
WEB_PORT=3000

DATABASE_URL=postgres://acf:acf@postgres:5432/acf?sslmode=disable
REDIS_URL=redis://redis:6379/0

WORKFLOW_PROVIDER=mock
CONTENT_PACKS=novel

OBJECT_STORAGE_DRIVER=local
OBJECT_STORAGE_PATH=/data/objects

LOG_LEVEL=info
```

P0 不得出现真实 Provider Key。

## 15. 命名规范

### Go

- Package：小写单词，不使用下划线。
- Entity：单数，如 `Material`。
- Command：动词开头，如 `CreateMaterialCommand`。
- Query：`GetMaterialQuery`、`ListProjectMaterialsQuery`。
- Handler：`CreateMaterialHandler`。
- Repository：`MaterialRepository`。

### API

```text
/projects
/projects/{projectId}/materials
/content-items/{contentItemId}/reviews/mock
```

- 资源使用复数名词。
- 动作接口仅用于无法自然表达的领域动作：`confirm`、`mock-generate`、`mock-rewrite`。
- ID 命名统一 `projectId`，生成代码内部按语言规范转换。

### TypeScript

- 文件：kebab-case。
- React 组件：PascalCase。
- Hook：`useXxx`。
- Query key：`projectKeys.detail(id)`。
- Frame ID：保持大写下划线，不作为文件名风格。

### 数据库

- 表名：snake_case 复数。
- 列名：snake_case。
- 外键：`<entity>_id`。
- 时间：`created_at`、`updated_at`、`deleted_at`（仅需要软删除时）。

## 16. Import 规则

### Go

- 模块内部允许引用自身 domain/application。
- 跨模块业务调用通过 application port 或明确的 read service。
- 禁止跨模块 import 对方 infrastructure。
- 禁止跨模块直接访问对方表。

### Web

- `shared` 不引用 `entities/features/widgets/app`。
- `entities` 不引用 `features/widgets/app`。
- `features` 不直接引用其他 feature 的内部文件。
- 跨 feature 共享能力下沉到 entity 或 shared。

## 17. Git 与迭代规范

- 每个小迭代验收通过后一个 commit。
- 不要求远端 push 才能完成迭代。
- Commit 前必须执行本迭代 verify。
- 禁止同时开发两个未完成迭代。
- 契约变化必须在 commit 中同时包含：
  - OpenAPI / Schema。
  - 后端实现。
  - 前端调用。
  - 测试。
  - 文档和追踪矩阵。

Commit 示例：

```text
feat: complete iteration 03 material lifecycle
```

## 18. 脚手架验收

Iteration 01 完成时必须满足：

```text
docker compose up
→ PostgreSQL / Redis / API / Worker / Web 正常启动
→ /healthz 200
→ /readyz 200
→ S00_HOME 可访问
→ migration 成功
→ seed 成功
→ Go test 通过
→ Web lint/typecheck 通过
→ OpenAPI 校验通过
→ Playwright 基础用例通过
```
