# P0 页面与闭环链路基线

## 全局入口

```text
S00_HOME
├── 首页 → S00_HOME
├── 项目 → S01_PROJECTS
├── 素材 → E1_GLOBAL_MATERIALS
├── 作品 → E2_GLOBAL_WORKS
├── 流程 → E3_WORKFLOWS
└── 设置 → E4_SETTINGS
```

## 项目创建

```text
S01_PROJECTS
├── 项目卡片 → S01_PROJECTS
├── 进入项目 → S02_PROJECT_OVERVIEW
└── 创建新项目 → S01_CREATE_PROJECT
    ├── 确认创建 → S02_PROJECT_OVERVIEW
    └── 取消 → S01_PROJECTS
```

## 项目工作区

```text
S02_PROJECT_OVERVIEW
├── 项目概览 → S02_PROJECT_OVERVIEW
├── 策划 → S02_PROJECT_PLANNING
├── 素材 → S02_PROJECT_MATERIALS
├── 故事线 → B1_STORYLINES
├── 章节 → C1_CHAPTER_PLANNING
├── 审核 → D2_REVIEW
├── 作品 → D3_PROJECT_WORKS
└── 设置 → P0 不展开
```

## 素材

```text
S02_PROJECT_MATERIALS
├── 新建素材 → S02_CREATE_MATERIAL → 返回 S02_PROJECT_MATERIALS
└── 素材详情 → S02_MATERIAL_DETAIL
    ├── 编辑素材 → S02_EDIT_MATERIAL → 返回 S02_MATERIAL_DETAIL
    ├── 编辑项目用途 → S02_EDIT_MATERIAL_USAGE → 返回 S02_MATERIAL_DETAIL
    ├── 选择已有素材 → S02_PICK_MATERIAL
    │   ├── 确认绑定 → S02_PROJECT_MATERIALS
    │   └── 取消 → S02_MATERIAL_DETAIL
    └── 解除绑定 → S02_UNBIND_MATERIAL
        ├── 确认解除 → S02_PROJECT_MATERIALS
        └── 取消 → S02_MATERIAL_DETAIL
```

## 故事线

```text
B1_STORYLINES
├── 新建主线 → B2_CREATE_MAIN_STORYLINE → B1_STORYLINES
├── 新建子故事线 → B3_CREATE_CHILD_STORYLINE → B1_STORYLINES
└── 新增伏笔 → B4_CREATE_FORESHADOWING → B1_STORYLINES
```

## 章节规划

开发语义采用：

```text
C1_CHAPTER_PLANNING
├── 模拟生成 → C2_MOCK_PLAN → C1_CHAPTER_PLANNING
├── 编辑规划 → C3_EDIT_PLAN → C1_CHAPTER_PLANNING
├── 确认规划 → C4_CONFIRM_PLAN → C1_CHAPTER_PLANNING
└── 已确认章节“进入正文生产” → D1_EDITOR
```

说明：冻结 UI 中 `C4_CONFIRM_PLAN` 是独立确认 Frame。实现必须保证“确认规划”和“进入正文生产”是两个状态动作；确认不自动生成正文。

## 正文、审核、重写、作品

```text
D1_EDITOR
├── 提交审核 → D2_REVIEW
├── 查看审核结果 → D2_REVIEW
└── 返回章节规划 → C1_CHAPTER_PLANNING

D2_REVIEW
├── 返回正文编辑器 → D1_EDITOR
├── 创建重写版本 → D4_CREATE_REWRITE
└── 返回项目作品 → D3_PROJECT_WORKS

D3_PROJECT_WORKS
├── 打开正文 → D1_EDITOR
├── 查看审核结果 → D2_REVIEW
├── 创建重写版本 → D4_CREATE_REWRITE
└── 返回项目概览 → S02_PROJECT_OVERVIEW

D4_CREATE_REWRITE
├── 确认创建 → D5_REWRITE_RESULT
└── 取消 → 来源页

D5_REWRITE_RESULT
├── 返回正文编辑器 → D1_EDITOR
├── 返回审核结果 → D2_REVIEW
└── 返回项目作品 → D3_PROJECT_WORKS
```

## 全局 Lite 页面

```text
E1_GLOBAL_MATERIALS → 引用项目 / 素材详情
E2_GLOBAL_WORKS → 源项目 / 正文 / 审核
E3_WORKFLOWS → 内置模拟流程产物
E4_SETTINGS → 能力与集成状态；查看内置流程可进入 E3_WORKFLOWS
```
