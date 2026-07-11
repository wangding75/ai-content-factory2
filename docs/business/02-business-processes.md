# AI Content Factory 2.0｜业务流程

## 1. 项目创建流程

```text
校验请求
→ 创建 Project(status=planning)
→ 写 AuditLog
→ 提交事务
→ 返回 projectId
→ 进入项目概览
```

失败要求：任何校验或数据库错误都不能留下部分 Project 或 AuditLog。

## 2. 项目内创建素材

```text
校验项目与素材类型
→ 创建 Material
→ 创建 ProjectMaterialUsage
→ 写 AuditLog
→ 同一事务提交
```

失败要求：Material 和 Usage 必须同时成功或同时回滚。

## 3. 绑定已有素材

```text
校验 Project 和 Material 存在
→ 校验未重复绑定
→ 创建 Usage
→ 写 AuditLog
```

## 4. Mock 生成章节候选

```text
创建 WorkflowRun(queued)
→ 标记 running
→ Mock Provider 执行
→ 创建 pending_confirmation ChapterPlan
→ WorkflowRun=succeeded
```

Provider 失败时 WorkflowRun=failed，不得创建部分候选。

## 5. 确认章节规划

```text
校验候选均为 pending_confirmation
→ 批量更新 confirmed
→ 写 confirmed_at
→ 写 AuditLog
```

确认不创建 ContentItem。

## 6. 创建正文

```text
校验 ChapterPlan=confirmed
→ 创建 ContentItem(draft)
→ 创建 ContentVersion(v1)
→ 设置 current_version_id
→ 写 AuditLog
```

同一章节规划不得重复创建多个主 ContentItem。

## 7. 审核流程

```text
锁定目标 ContentVersion
→ 创建 WorkflowRun
→ Mock Review
→ 创建 ReviewReport + Findings + Recommendations
→ 更新 ContentItem.review_status
```

审核结果必须关联固定 versionId，之后正文变化不影响历史报告。

## 8. 重写流程

```text
选择源版本和审核建议
→ 创建 WorkflowRun
→ Mock Rewrite
→ 创建 ContentVersion(vN+1, parent_version_id=source)
→ 保留 current_version_id 不变
→ 返回重写结果
```
