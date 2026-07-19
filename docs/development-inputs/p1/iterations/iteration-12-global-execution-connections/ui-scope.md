# Iteration 12 — 全局配置中心 V2 — UI Scope

## 1. 信息架构

```text
全局设置
├─ LLM 配置
│  ├─ 空状态
│  ├─ 列表态
│  └─ 添加/编辑抽屉
├─ 工作流配置
│  ├─ 连接
│  │  ├─ 空状态
│  │  ├─ 列表态
│  │  └─ 添加/编辑抽屉
│  └─ 工作流
│     ├─ 空状态
│     ├─ 列表态
│     └─ 添加/编辑抽屉
└─ 分发平台配置
   ├─ 空状态
   ├─ 列表态
   └─ 添加/编辑抽屉
```

## 2. 原型关联

| Frame | 区域 | 用途 | 原始目录 | 截图 | HTML |
|---|---|---|---|---|---|
| `GLOBAL_SETTINGS_LLM_EMPTY_V2` | 全局设置 / LLM 配置 | 首次进入 LLM 配置时的空状态与添加入口 | `llm_2` | `ui/frames/GLOBAL_SETTINGS_LLM_EMPTY_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_LLM_EMPTY_V2/code.html` |
| `GLOBAL_SETTINGS_LLM_LIST_V2` | 全局设置 / LLM 配置 | 搜索、筛选和管理 LLM Provider 配置 | `llm_v2` | `ui/frames/GLOBAL_SETTINGS_LLM_LIST_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_LLM_LIST_V2/code.html` |
| `LLM_CONFIG_DRAWER_V2` | 全局设置 / LLM 配置 | 新增、编辑、验证 LLM Provider | `llm_1` | `ui/frames/LLM_CONFIG_DRAWER_V2/screen.png` | `ui/frames/LLM_CONFIG_DRAWER_V2/code.html` |
| `GLOBAL_SETTINGS_CONNECTION_EMPTY_V2` | 全局设置 / 工作流配置 / 连接 | 首次进入连接管理时的空状态与添加入口 | `v2_4` | `ui/frames/GLOBAL_SETTINGS_CONNECTION_EMPTY_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_CONNECTION_EMPTY_V2/code.html` |
| `GLOBAL_SETTINGS_CONNECTION_LIST_V2` | 全局设置 / 工作流配置 / 连接 | 搜索、筛选和管理工作流连接 | `v2_1` | `ui/frames/GLOBAL_SETTINGS_CONNECTION_LIST_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_CONNECTION_LIST_V2/code.html` |
| `CONNECTION_DRAWER_V2` | 全局设置 / 工作流配置 / 连接 | 新增、编辑、验证通用工作流连接 | `v2_5` | `ui/frames/CONNECTION_DRAWER_V2/screen.png` | `ui/frames/CONNECTION_DRAWER_V2/code.html` |
| `GLOBAL_SETTINGS_WORKFLOW_EMPTY_V2` | 全局设置 / 工作流配置 / 工作流 | 首次进入工作流资源管理时的空状态与添加入口 | `v2_8` | `ui/frames/GLOBAL_SETTINGS_WORKFLOW_EMPTY_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_WORKFLOW_EMPTY_V2/code.html` |
| `GLOBAL_SETTINGS_WORKFLOW_LIST_V2` | 全局设置 / 工作流配置 / 工作流 | 搜索、筛选和管理可供项目绑定的工作流资源 | `v2_2` | `ui/frames/GLOBAL_SETTINGS_WORKFLOW_LIST_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_WORKFLOW_LIST_V2/code.html` |
| `WORKFLOW_DRAWER_V2` | 全局设置 / 工作流配置 / 工作流 | 新增、编辑、验证可复用工作流资源 | `v2_7` | `ui/frames/WORKFLOW_DRAWER_V2/screen.png` | `ui/frames/WORKFLOW_DRAWER_V2/code.html` |
| `GLOBAL_SETTINGS_DISTRIBUTION_EMPTY_V2` | 全局设置 / 分发平台配置 | 首次进入分发平台配置时的空状态与添加入口 | `./ (archive root)` | `ui/frames/GLOBAL_SETTINGS_DISTRIBUTION_EMPTY_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_DISTRIBUTION_EMPTY_V2/code.html` |
| `GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2` | 全局设置 / 分发平台配置 | 搜索、筛选和管理内容分发平台配置 | `v2_6` | `ui/frames/GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2/screen.png` | `ui/frames/GLOBAL_SETTINGS_DISTRIBUTION_LIST_V2/code.html` |
| `DISTRIBUTION_DRAWER_V2` | 全局设置 / 分发平台配置 | 新增、编辑、验证分发平台连接 | `v2_3` | `ui/frames/DISTRIBUTION_DRAWER_V2/screen.png` | `ui/frames/DISTRIBUTION_DRAWER_V2/code.html` |

## 3. 页面壳层规则

原型内侧边栏和页面头部存在多套视觉版本。开发时：

- 复用仓库现有全局 SideNav、TopBar、页面标题区、用户信息和面包屑；
- 不根据每张原型重写页面壳层；
- 只开发页面标题下方工作区；
- LLM、连接、工作流和分发平台使用同一内容宽度、Tab、表格、空状态和抽屉组件；
- 抽屉背景是当前真实父列表页，原型中的错误背景只作为已知问题记录。

## 4. 按钮位置

所有空状态和列表状态统一：

- 主添加按钮固定在工作区标题栏右上角；
- 空状态卡片内部不再重复放置添加按钮；
- 从空状态切换到列表态时，添加按钮位置保持不变。

## 5. 连接与工作流规则

- 工作流配置下二级 Tab 统一为“连接 / 工作流”；
- 页面标题和按钮使用“连接”，不写死 n8n；
- 新增连接时“连接类型”可选择，当前只有 n8n；
- 编辑连接时“连接类型”只读；
- 工作流表单先选择连接，再只读展示推导类型；
- 工作流列表展示关联连接和连接类型；
- 更换关联连接后，工作流类型及专属字段自动更新。

## 6. 文案和安全

- 所有用户可见业务文案使用中文 locale/i18n 资源；
- `Workflow ID`、Webhook Path、模型名和 Schema 版本可以保留英文技术标识；
- 密钥和凭证不回显；
- 错误提示必须脱敏；
- 原型 HTML 仅是视觉参考，不是可直接复制的生产代码。

## 7. Iteration 12 验证和启用控件

原型包含验证按钮、验证状态和启用开关，但本迭代不访问真实第三方：

- 列表状态统一显示“未接入”；
- 验证按钮禁用；
- 启用开关禁用；
- 悬浮或辅助文案说明“将在后续适配迭代开放”；
- 不使用 Mock 将配置伪装为验证成功；
- 不产生虚假的最近验证时间；
- `integrationStatus` 和 `enabled` 由后续迭代激活。

## 8. 条件通过

本组原型不再要求重新生成。P0-1、P0-2、P0-3、页面壳层差异、重复按钮和第三方控件延期均作为开发强制规则。
