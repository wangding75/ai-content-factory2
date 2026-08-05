# 评审结论与失效声明 (Review Result)

## 一、旧采集失效说明
本目录下的所有 UI 截图证据及 `collection-manifest.json`、`collection-summary.md` 等相关交付物已正式声明**完全失效**。

## 二、失效原因
- 包含了通过 `page.route` 和 API Mock 拦截返回的伪造数据强制修改的页面状态；
- 部分被标记为成功的收集项的 target_state_visible 判定不成立；
- 重复利用并复制了相同的页面截图；
- 本地有些特定加载中或卡死页面被错误标记为成功采集。

## 三、强制性要求
- **本目录下的所有文件不得用于第二阶段的精确 UI 比对验收。**
- 新的一手无 Mock、纯实盘无改动、完全基于真实后端数据库流转生成的真实 UI 证据已保存在新目录：
  [docs/acceptance/second-loop-ui/phase-1-collection-v2/](../phase-1-collection-v2/)
- 本目录下的原有旧截图、Manifest 等历史文件保持现状，不做任何人为修改、删除或覆盖，作为失效历史记录保留。
