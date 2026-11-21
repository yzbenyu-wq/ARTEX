# 决策卡：资产测绘 / 种子闭环

## 何时用
- 只有集团名/品牌名，没有 URL 清单。
- 需要子域、FOFA、端口、指纹、bbot、OSINT。
- 挖完一个种子，犹豫要不要换。

## 首打 5 步
1. 种子来源：业务名 + 全资 1～4 级子公司 + 根域/备案 + 品牌别名。
2. 落盘 `资产/种子队列.md`；数量不设上限，不准砍成固定条数。
3. FOFA 只作雷达：最短语法在 `kb:recon-methodology` 文首；搜资产走 MCP，不自己 curl。
4. 清洗：去重、去废、去非存活；过归属/股权闸（参股默认不挖）。
5. 一种子剩余活面挖完，才换下一条；全 done 未叫停就回扫旧种子只捞新增。

## 判定
- 一个 host 是否有可挖业务面：网关、跳转后业务 host、业务 API。
- 优质根域回灌：从 host 向上归约到业务簇根/品牌根，再搜 `domain=R`。

## 常见坑
- 多种子一次搜完再挖。
- 把 FOFA 条数、nuclei 全量扫描当进度。
- 登录壳同皮反复挖；只留 2～3 个代表。
- 429 时硬刚；应只换池。

## 主源
- `rule:dig-scope-workflow`：§1 自由跳、§3 去废/归属/股权闸
- `kb:recon-methodology`：FOFA 最短语法、§2 子域枚举
- `skill:bbot-recon`：被动零接触 / 主动模块的策略闸
- `skill:offensive-osint`

## 打开
```bash
python3 scripts/index.py show kb:recon-methodology --section "FOFA 最短语法"
python3 scripts/index.py show rule:dig-scope-workflow --section "1. 自由跳"
python3 scripts/index.py show skill:bbot-recon
```
