# 验收方案

| 用例 ID | 场景 | 通过标准 |
|---|---|---|
| I08-AC01 | 全局素材聚合 | 项目内创建的素材在 E1 可见，并展示正确引用项目与用途摘要。 |
| I08-AC02 | 全局作品聚合 | E2 可跨项目展示作品，并能定位到源项目、正文和审核。 |
| I08-AC03 | 流程中心边界 | E3 只显示内置模拟流程，不执行 n8n/Coze/ComfyUI。 |
| I08-AC04 | 设置状态准确 | 模拟能力=已启用；真实 AI=暂未配置；发布与外部工作流=暂未开放。 |
| I08-AC05 | 无虚假配置 | P0 页面不得出现 API Key、OAuth、连接成功或真实调用记录。 |
| I08-AC06 | Lite route/action contract | E1--E4 use the four frozen top-level routes; every enabled action has an existing route/API and all required identifiers. |

## 门禁

- E1--E4 manifest routes are exactly `/materials`, `/works`, `/workflows`, and `/settings`; each frozen HTML top navigation uses those routes.
- Every visible Lite link has an existing route and required identifier contract, or is disabled/removed. No frozen HTML contains `href="#"`.
- List requests define loading, success, empty and safe-error states; list pagination uses `limit`, `offset` and its documented stable order.

- 核心用例必须可重复执行。
- 不接受仅页面打开或 HTTP 200 的冒烟结果。
- 失败分支必须验证数据库无脏数据。
