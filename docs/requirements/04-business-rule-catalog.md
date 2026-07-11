# P0 业务规则映射

完整规则正文见：

```text
docs/business/03-business-rules.md
```

状态机见：

```text
docs/development-inputs/p0/baselines/p0-status-machines.md
```

实现要求：

- Domain 单元测试按业务规则 ID 命名或在测试描述中引用规则 ID。
- API 错误响应应能区分 validation、not_found、conflict 和 invalid_state。
- E2E 至少覆盖素材解绑、规划确认、审核不覆盖正文和重写保留旧版本。
