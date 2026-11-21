# 决策卡：新目标 / 进站

## 何时用
- 新目标、只给集团名、给 URL/清单、说“挖某某”。
- 不知道该先打哪、要不要继续、什么时候换站。

## 首打 5 步
1. 判模式：固定站/文档/清单 = 锁面；模糊目标 = 自由跳。
2. 自由跳先落 `资产/种子队列.md`：业务名 + 全资子公司域 + 根域/备案。
3. 一种子闭环：搜一个种子 → 去重去废去非存活 → 活面挖完 → 标 done → 再搜下一条。
4. 进站先看短表；有前端抽 JS，建 path + 钥匙清单；对得上再开模块。
5. 每轮结束读种子队列；有 pending 不许用问句停住。

## 判定
- 只打授权域名/IP/账号；范围外不碰。
- 写越权最小伤害；禁登出；CORS 不挖。
- 高价值苗头先打穿；中危同一对象先升链。

## 常见坑
- 多种子一次搜完再挖，违反一种子闭环。
- 拿 FOFA 条数当收工。
- 看见登录页就换资产；应先找业务面/业务 API。
- 每站通读整个知识库。

## 主源
- `src:entry`（总入口）
- `rule:dig-scope-workflow`：§0 先判模式、§1 自由跳、§4 单站主路径
- `rule:src-value-hunting`：§1 挖什么、§3 类型矩阵
- `rule:hunt-iter`：短表迭代

## 次源
- `kb:recon-methodology`
- `kb:js-reverse-guide`

## 打开
```bash
python3 scripts/index.py show src:entry
python3 scripts/index.py show rule:dig-scope-workflow --section "0. 先判模式"
python3 scripts/index.py show rule:dig-scope-workflow --section "4. 单站主路径"
```
