# 页面与原型对照（最新）

| 项 | 内容 |
|---|---|
| 检查时间 | 2026-07-18 |
| 环境 | Docker Compose：Web `http://localhost:13001` / API `http://localhost:18080` |
| 源码 | `b40545a` |
| 演示项目 | 末日求生（`8e16e531-cdb0-44d2-b4c6-5d47a8abdec7`） |
| 实机截图 | [screenshots/live/](./screenshots/live/) |
| 原型截图 | [screenshots/prototype/](./screenshots/prototype/) |

> 13/13 核心页 HTTP 200，API 联通。空项目/空列表与原型填充态差异不一律记为功能缺失。

---

## 目录

```text
docs/ui-prototype-comparison/
├── README.md                 # 本报告
└── screenshots/
    ├── live/                 # 最新实机截图
    └── prototype/            # 冻结 Frame 原型截图
```

---

## 与原型差异大的部分

### 很大（形态 / 信息架构）

| 页面 | 原型 | 实机 |
|------|------|------|
| **项目列表** | 胶囊筛选：全部/策划中/生产中/审核中 | 下拉 +「筛选」；状态为制作中/已归档 |
| **新建项目** | 列表上的模态 | 独立全页 `/projects/new` |
| **流程中心** | 中文名、步骤、顶部统计 | 英文标题、key/provider 元数据 |
| **设置** | 中文能力名、Tab 结构 | 英文 + `enabled true/false` 工程字段 |
| **全局素材** | 统计条 + 类型胶囊 + 引用筛选 | 仅类型下拉 |
| **正文编辑器** | 三栏，右侧目标/故事线/素材/伏笔 | 右侧上下文弱/缺（本轮未有数据深截） |

### 中等

| 页面 | 主要差距 |
|------|----------|
| **章节规划** | 缺「新增章节」、模拟说明条；无「已审核」Tab |
| **项目卡片** | 渐变占位封面；进度写死「待开始」 |
| **项目概览** | 缺主题/风格/受众块；视觉/活动多为空 |
| **项目作品** | 缺状态胶囊、双栏详情、交付资产（空态正常） |
| **故事线** | 骨架接近；有数据时密度仍低于原型 |

### 较小

- 首页结构对齐，封面精细度略低  
- 项目素材 Tab 形态接近  
- 品牌副标题、用户区、「章节」vs「章节规划」文案  

---

## 建议优先修

1. 流程中心 / 设置中文化  
2. 项目列表胶囊筛选 + 新建交互形态  
3. 全局素材统计与类型胶囊  
4. 正文编辑器右侧上下文  
5. 章节规划 CTA / 说明  

---

## 截图对照

| Frame | 原型 | 实机 |
|-------|------|------|
| S00 首页 | [prototype](./screenshots/prototype/S00_HOME.png) | [live](./screenshots/live/S00_HOME.png) |
| S01 项目列表 | [prototype](./screenshots/prototype/S01_PROJECTS.png) | [live](./screenshots/live/S01_PROJECTS.png) |
| S01 新建 | [prototype](./screenshots/prototype/S01_CREATE_PROJECT.png) | [live](./screenshots/live/S01_CREATE_PROJECT.png) |
| S02 概览 | [prototype](./screenshots/prototype/S02_PROJECT_OVERVIEW.png) | [live](./screenshots/live/S02_PROJECT_OVERVIEW.png) |
| S02 素材 | [prototype](./screenshots/prototype/S02_PROJECT_MATERIALS.png) | [live](./screenshots/live/S02_PROJECT_MATERIALS.png) |
| B1 故事线 | [prototype](./screenshots/prototype/B1_STORYLINES.png) | [live](./screenshots/live/B1_STORYLINES.png) |
| C1 章节规划 | [prototype](./screenshots/prototype/C1_CHAPTER_PLANNING.png) | [live](./screenshots/live/C1_CHAPTER_PLANNING.png) |
| D1 正文（仅原型） | [prototype](./screenshots/prototype/D1_EDITOR.png) | — |
| D3 作品 | [prototype](./screenshots/prototype/D3_PROJECT_WORKS.png) | [live](./screenshots/live/D3_PROJECT_WORKS.png) |
| E1 全局素材 | [prototype](./screenshots/prototype/E1_GLOBAL_MATERIALS.png) | [live](./screenshots/live/E1_GLOBAL_MATERIALS.png) |
| E3 流程 | [prototype](./screenshots/prototype/E3_WORKFLOWS.png) | [live](./screenshots/live/E3_WORKFLOWS.png) |
| E4 设置 | [prototype](./screenshots/prototype/E4_SETTINGS.png) | [live](./screenshots/live/E4_SETTINGS.png) |

另有实机：`S02_PROJECT_PLANNING.png`、`E2_GLOBAL_WORKS.png`。

---

## 备注

- 列表中曾有编码损坏项目名 `????`，建议删除。  
- 概览「故事线节点」可能与故事线页计数不一致，待核。  
- D1/D2/D4/D5 需有章节正文后再补实机截图。  
