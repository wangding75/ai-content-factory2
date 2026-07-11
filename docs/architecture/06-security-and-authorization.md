# 安全与权限基线

## 1. P0 边界

P0 是单用户开发基线，不实现登录和租户隔离。但所有实体和审计模型应保留未来 actor/workspace 边界，避免后续大规模重构。

## 2. 输入安全

- 所有枚举、长度和结构在边界校验。
- JSONB payload 由 Content Pack Schema 校验。
- SQL 全部参数化。
- 正文默认按文本安全渲染；引入富文本前需单独威胁评审。

## 3. 密钥与配置

- `.env` 不提交 Git。
- 日志不输出密码、令牌或未来 Provider Key。
- 设置页不得返回密钥明文。

## 4. 未来权限边界

建议资源层级：

```text
Workspace → Project → Project resources
Global asset scope is workspace-scoped
```

未来授权必须在 Application 层统一检查，而不是散落在页面和 Handler。
