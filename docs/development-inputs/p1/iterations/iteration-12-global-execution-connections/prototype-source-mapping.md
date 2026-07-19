# Iteration 12 原型来源映射

## 1. 来源

- 上传包：`stitch_ai_content_factory_2.0_v2 (1).zip`
- SHA-256：`da2258504e1ed3ffd7114f598d4471ce2c83785f3204a0e94d9cdcfd02a23d7c`
- Stitch 项目：`AI Content Factory 2.0｜全局设置 V2`
- 原始目录命名不具备开发语义，本包已规范化为 Frame ID。

## 2. 映射

| Frame ID | 页面 | 原始目录 | 状态 | 父页面/壳层 |
|---|---|---|---|---|
| `GLOBAL_SETTINGS_LLM_EMPTY_V2` | 全局设置 - LLM 配置空状态 | `llm_2` | `EMPTY` | `GLOBAL_SETTINGS_SHELL` |
| `GLOBAL_SETTINGS_LLM_LIST_V2` | 全局设置 - LLM 配置列表 | `llm_v2` | `LIST` | `GLOBAL_SETTINGS_SHELL` |
| `LLM_CONFIG_DRAWER_V2` | 添加/编辑 LLM 配置抽屉 | `llm_1` | `DRAWER` | `GLOBAL_SETTINGS_LLM_LIST_V2` |
| `GLOBAL_SETTINGS_CONNECTION_EMPTY_V2` | 全局设置 - 连接空状态 | `v2_4` | `EMPTY` | `GLOBAL_SETTINGS_SHELL` |
| `GLOBAL_SETTINGS_CONNECTION_LIST_V2` | 全局设置 - 连接列表 | `v2_1` | `LIST` | `GLOBAL_SETTINGS_SHELL` |
| `CONNECTION_DRAWER_V2` | 添加/编辑连接抽屉 | `v2_5` | `DRAWER` | `GLOBAL_SETTINGS_CONNECTION_LIST_V2` |
| `GLOBAL_SETTINGS_WORKFLOW_EMPTY_V2` | 全局设置 - 工作流空状态 | `v2_8` | `EMPTY` | `GLOBAL_SETTINGS_SHELL` |
| `GLOBAL_SETTINGS_WORKFLOW_LIST_V2` | 全局设置 - 工作流列表 | `v2_2` | `LIST` | `GLOBAL_SETTINGS_SHELL` |
| `WORKFLOW_DRAWER_V2` | 添加/编辑工作流抽屉 | `v2_7` | `DRAWER` | `GLOBAL_SETTINGS_WORKFLOW_LIST_V2` |
| `GLOBAL_SETTINGS_DISTRIBUTION_EMPTY_V2` | 全局设置 - 分发平台配置空状态 | `./ (archive root)` | `EMPTY` | `GLOBAL_SETTINGS_SHELL` |
| `GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2` | 全局设置 - 分发平台配置列表 | `v2_6` | `LIST` | `GLOBAL_SETTINGS_SHELL` |
| `DISTRIBUTION_DRAWER_V2` | 添加/编辑分发平台配置抽屉 | `v2_3` | `DRAWER` | `GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2` |

## 3. 使用规则

- 开发只引用规范化后的 `ui/frames/<FRAME_ID>`；
- 原始 `v2_1`、`llm_1` 等名称仅用于追溯；
- 原型问题不在 HTML 中直接修补，而是以 `prototype-review-and-development-fixes.md` 和 `acceptance.md` 约束生产实现；
- `source/` 下原始 ZIP 只随交付包保存，安装脚本不会将其复制进仓库。
