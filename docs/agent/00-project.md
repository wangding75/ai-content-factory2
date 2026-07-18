# 项目固定信息

## 项目

- 名称：AI Content Factory 2.0
- 本地仓库：`D:\github\ai-content-factory2`
- 默认分支：`main`
- 状态文件：`.ai-dev/state.json`
- 当前研发方式：按独立任务执行，每个任务独立 commit、push、评审后再进入下一任务

执行 Agent 不得自行创建新的计划阶段或自动开始下一任务。

## 权威来源

发生信息冲突时按以下优先级判断：

1. 当前任务指令中的明确特殊约束
2. 已冻结的 OpenAPI、Schema 与契约文件
3. 已冻结 UI HTML、截图和对应验收资料
4. 当前迭代 scope、acceptance、closed-loop 等文档
5. 现有实现
6. 历史说明或推测

不得以当前页面代替冻结原型，不得以 Mock 行为覆盖正式契约。

## 运行环境

- 操作系统：Windows
- Shell：PowerShell
- Codex：可信仓库下使用非沙箱模式
- pnpm：统一调用 `pnpm.cmd`，不得调用可能被 PowerShell 执行策略拦截的 `pnpm.ps1`
- Docker Compose 项目：`ai-content-factory2`
- Web：`http://127.0.0.1:13001`
- 数据库：`ai_content_factory`
- 持久化卷逻辑名：`postgres_data`
- 实际 Compose 卷名：`ai-content-factory2_postgres_data`（由 Compose 项目名自动添加前缀；以 `docker compose volumes` 或 `docker volume ls` 的实际输出为准，不得把该名称硬编码为跨项目固定值）

端口、服务名或脚本若与仓库当前配置不一致，以仓库配置为准，并在回执中说明。

## 数据库固定策略

所有后续迭代复用同一个开发数据库和持久化卷。

禁止：

- 按迭代创建新的空数据库
- 正常开发过程中清空数据库
- 删除数据库持久化卷
- 执行 `docker compose down -v`
- 未经任务明确授权修改 Migration
- 未经任务明确授权覆盖现有数据

统一 UI 回归数据应由单一、幂等、可重复执行的 Seed SQL 管理；具体文件由对应任务创建。

## 固定 Git 策略

每个任务：

1. 从已确认的基线提交开始
2. 只修改当前任务范围
3. 独立 commit
4. push 到远端
5. 返回完整 commit hash
6. 等待评审后再进入下一任务

禁止：

- `git reset`
- `git clean`
- `git restore`
- `git checkout` 用于丢弃修改
- `git stash`
- amend
- rebase
- squash
- force push

发现无法解释的已有修改时停止并报告，不得擅自删除。

## 固定产品展示约束

面向用户的 UI 默认不得直接展示：

- UUID
- key
- provider
- 数据库字段名
- 原始英文枚举
- ISO 时间
- JSON
- Prompt
- 模型参数
- 内部配置对象
- `true` / `false`
- `[object Object]`

必须通过集中 Mapper 转换为用户可读内容；未知值使用安全兜底，不直接回显原值。
