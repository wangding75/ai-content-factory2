# Iteration 18 — 真实正文重写验收标准

**状态：`rebuild_candidate_2026_07_29`。** 本次临时任务只验收文档/原型替换包；以下业务矩阵供 CF-18-01 冻结，不代表当前代码已实现。

## 1. 文档与原型替换验收

- [ ] 9 个 Frame 均存在 1600×1280 screen.png 与非空 code.html。
- [ ] ui-manifest.json 为 UTF-8、合法 JSON、frameCount=9，路径均存在。
- [ ] ui-scope、iteration-plan、prototype mapping 和 UI/API 追踪包含全部 9 Frame。
- [ ] 旧 5 Frame 目录从 Iteration 18 中删除；不修改 Iteration 15/16/17、OpenAPI、Migration、代码或 n8n。
- [ ] p1/ui-master-manifest.json 和 ui-version-selection.md 只更新 Iteration 18 条目。
- [ ] Git diff 只包含替换清单文件，`git diff --check` 通过。

## 2. CF-18-01 业务冻结矩阵

| 范围 | PASS 条件 |
|---|---|
| Stage/Subject | content_rewrite + review_report 唯一语义，无平行 Runtime |
| 来源 | Report、Issue、来源版本、Run、候选和 IssueLink 完整追踪 |
| Issue | 只允许同 Report 的 open Issue；1～50 项；不自动改 disposition |
| 输入输出 | rewrite.input.v1 / rewrite.output.v1 严格 Schema |
| Preflight/Create | 只读预检、Token、幂等、事务和 active Run 并发完整 |
| 消费 | 候选与全部 IssueLink 原子写入；失败零部分数据 |
| 状态 | 8 个 Summary 状态及优先级完整 |
| Retry | Runtime Retry 与仅消费 Retry 严格区分 |
| 候选 | 非当前、不覆盖源版本、不自动审核/发布 |
| Set Current | 复用 Iteration 16 CAS，stale 不强制覆盖 |
| P0 | Mock Rewrite 兼容且来源标识清晰 |
| UI | 9 个 Frame 与 API/状态/模型可追踪 |

## 3. 不在本次替换任务验收范围

- OpenAPI、Migration、后端、前端或 n8n 实现；
- 浏览器实机截图；
- Iteration 17 冻结修复；
- Iteration 19 集成验收。
