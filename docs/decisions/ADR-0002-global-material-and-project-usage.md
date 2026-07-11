# ADR-0002：全局素材本体与项目用途分离

- 状态：Accepted
- 决策：Material 全局唯一；ProjectMaterialUsage 表示项目关系。
- 原因：支持跨项目复用，同时避免项目特定角色污染全局资产。
- 后果：编辑素材本体具有全局影响；解绑不能删除 Material。
