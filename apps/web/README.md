# ACF Web

`apps/web` 是 AI Content Factory 2.0 的 Next.js Web 应用，负责项目工作区、素材与故事线管理、章节规划、正文生产、内容审核、内容重写、全局配置、工作流绑定和 WorkflowRun 追踪。

当前分支已完成 Iteration 15～18 的页面实现和浏览器 UI 验收。该验收不代表真实 n8n 与真实 LLM 的最终端到端闭环已经通过。

## 技术要求

- Node.js：使用满足当前 Next.js 16 要求的版本
- 包管理器：`pnpm 9.1.1`
- 框架：Next.js 16、React 19、TypeScript 5

建议从仓库根目录执行命令，避免使用 npm、Yarn 或 Bun 生成额外锁文件。

## 安装依赖

在仓库根目录执行：

```bash
pnpm install
```

## 开发启动

从仓库根目录：

```bash
pnpm dev:web
```

或在当前目录：

```bash
pnpm dev
```

直接运行 Next.js 开发服务器时默认地址为 `http://localhost:3000`。通过仓库根目录的 Docker Compose 启动时，Web 地址为 `http://localhost:13001`。

## 环境变量

服务端 API 地址按以下优先级读取：

1. `API_BASE_URL`
2. `NEXT_PUBLIC_API_BASE_URL`
3. 默认值 `http://localhost:18080/api/v1`

浏览器端请求统一使用同源路径 `/api/v1`，由 Next.js/部署环境转发到 Go API。

Docker Compose 当前使用：

```text
API_BASE_URL=http://api:8080/api/v1
```

## 常用命令

```bash
pnpm dev
pnpm test
pnpm typecheck
pnpm lint
pnpm build
pnpm start
pnpm test:e2e
```

命令说明：

| 命令 | 作用 |
|---|---|
| `pnpm dev` | 启动 Next.js 开发服务器 |
| `pnpm test` | 运行 `src/**/*.test.ts` 逻辑测试 |
| `pnpm typecheck` | 执行 TypeScript 类型检查 |
| `pnpm lint` | 执行 ESLint |
| `pnpm build` | 生成生产构建 |
| `pnpm start` | 启动已构建的生产服务 |
| `pnpm test:e2e` | 执行 Playwright E2E |

当前 `pnpm test` 只匹配 `.test.ts`，不包含 `.test.tsx` 组件测试。不得在验收报告中把该命令描述为完整组件测试。

## 主要目录

### `src/app`

包含主要路由：

- `projects`
- `materials`
- `works`
- `workflows`
- `workflow-runs`
- `settings`
- `chapter-plan-candidate-batches/[batchId]`

### `src/features`

按业务能力组织页面和交互：

- `chapter-plans`
- `content-items`
- `content-review`
- `global-config`
- `global-lite`
- `planning-materials`
- `project-works`
- `storylines`
- `workflow-bindings`
- `workflow-runs`

## 与后端的关系

Web 通过 `/api/v1` 调用 Go API。业务请求必须遵守 OpenAPI、错误 Envelope、幂等键和版本控制要求。真实章节规划、正文生成、审核和重写应使用各自的业务接口，不能由通用 WorkflowRun 创建接口替代。

## 当前验收边界

Iteration 15～18 已完成浏览器 UI 对比验收，包括页面展示、主要交互、控制台检查、类型检查、Lint 和生产构建。以下能力仍不应标记为已完成：

- 真实 LLM Provider 端到端调用
- 四阶段真实 n8n 最终联调
- Iteration 19 最终异常恢复和关闭验收
- 完整前端组件测试覆盖
