# Iteration 19 — 第二用户闭环联调与关闭 — 数据模型

## 涉及模型

- `全部第二闭环模型`

## 通用约束

- 所有配置模型包含 `id/version/createdAt/updatedAt`。
- 密钥仅保存加密值和指纹，DTO 不返回密文。
- WorkflowRun 保存绑定快照，不受后续配置变更影响。
- 领域结果只有在输出 Schema 校验和事务提交成功后落库。
- 正文生成与重写始终创建新 ContentVersion。
- 审核始终绑定固定 ContentVersion。
