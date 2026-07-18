# 提交与验收规范

## 1. 提交前范围检查

提交前必须执行并检查：

```powershell
git diff --name-status
git diff --stat
git diff --check
git ls-files --others --exclude-standard
git status --short --untracked-files=all
```

确认：

- 只有当前任务范围内文件
- 无意外未跟踪文件
- 无 EOF 多余空行、尾随空格或冲突标记
- 无无关文档、依赖、契约、Migration 或状态文件变化

发现超范围修改时不得提交。

## 2. 机器验收

按以下顺序执行：

1. 环境预检
2. 最小定向测试
3. 相关模块测试
4. typecheck
5. 修改文件定向 lint
6. production build
7. `git diff --check`
8. Docker production 验证
9. 浏览器或接口专项验证
10. 提交前范围复核

固定准备工作只执行一次；失败后优先局部验证，不重复进行无意义的完整准备。

## 3. Docker production 验证

通常使用仓库现有 Compose 配置执行：

```powershell
docker compose up -d --build --remove-orphans
docker compose ps
docker compose logs --since=10m migrate api web
```

实际服务名以仓库配置为准。

必须确认：

- 数据库正常
- Migration 正常
- API healthy
- Web 可访问
- 未删除现有数据卷

禁止执行：

```powershell
docker compose down -v
```

## 4. 浏览器验收

UI 任务必须使用实际浏览器打开对应路由，至少检查：

- 页面可访问
- 核心结构与冻结原型一致
- 真实 API 数据正常
- 主要操作可用
- loading、empty、error、retry 正常
- Console 无 error
- 无 hydration error
- 无资源 404
- 无未处理 Promise rejection
- 无异常高频请求
- 无横向溢出
- 不泄露技术字段

当前任务指令只需补充本页面特有验收点。

滚动页面、抽屉或弹窗必须滚动到底后再判定内容缺失。

## 5. 真实 API 联调

正式验收不得仅依赖 Mock。

若任务涉及现有正式 API：

- 必须使用真实 API 验证
- 真实写操作必须验证成功与失败路径
- 不得用静态数据掩盖接口问题

Mock 仅用于可控状态测试，不替代 production 联调。

## 6. 人工验收与机器验收

机器验收负责：

- 测试
- 类型
- 构建
- 服务健康
- 路由可达
- DOM 和交互
- Console 和网络异常
- 明确规则校验

人工验收负责：

- 视觉观感
- 信息密度
- 交互合理性
- 与冻结稿的最终主观一致性

机器验收通过不代表 UI 已获人工冻结。

## 7. Commit 与 push

每个任务必须独立提交。

标准流程：

```powershell
git add --all
git diff --cached --name-status
git diff --cached --stat
git commit -m "<符合任务内容的提交信息>"
git push origin main
```

禁止：

- amend
- rebase
- squash
- force push
- 将多个已独立评审任务合并提交

push 失败或缺少完整 commit hash 时，不得声明任务完成。

## 8. 最终状态

完成任务后必须确认：

```powershell
git status --short --untracked-files=all
```

工作区必须 clean。

不得在同一任务中自动开始下一任务。

## 9. 状态判定

### PASS

同时满足：

- 业务目标完成
- 任务范围正确
- 固定门禁通过
- production 验证通过
- commit 成功
- push 成功
- 返回完整 commit hash
- 工作区 clean

### NEEDS CHANGES

代码已提交但存在可定位、可修复的问题，例如：

- 功能或 UI 与要求不一致
- 测试覆盖不足
- 技术字段泄露
- 超范围实现
- 验收证据不完整

下一步只下发定点修复任务。

### BLOCKED

因外部条件无法继续，例如：

- 权限
- 基础环境
- 外部服务
- 契约冲突
- 需要产品决策
- 需要破坏性操作授权

必须提供原始错误、受影响范围和已完成状态，不得提交未验证结果。

## 10. 最终回执模板

```text
状态：PASS / NEEDS CHANGES / BLOCKED

Commit：
- Hash：
- Message：
- Push：

范围：
- 修改文件：
- 新增文件：
- 删除文件：

验收：
- 定向测试：
- 相关模块测试：
- Typecheck：
- 定向 lint：
- Production build：
- git diff --check：
- Docker：
- 浏览器/API：

环境：
- Branch：
- Database：
- Volume：
- Workspace：

时间：
- 开始：
- 结束：
- 耗时：

阻塞或遗留：
- 无 / <具体内容>
```

回执只报告实际执行结果，不得补写未执行的验证。
