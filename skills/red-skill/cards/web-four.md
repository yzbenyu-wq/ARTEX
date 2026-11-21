# 决策卡：注入 / SSRF / XSS / RCE

## 何时用
- 有搜索/筛选、URL 参数、富文本、文件预览、URL 抓取、命令面。
- 想判断先打哪一类，避免每个 path 都喷引号。

## 首打 4 步
1. 先找有差分面的参：回显变化、报错、状态码、时延、DNSLog。
2. 按栈选探针：SQL/NoSQL、模板、命令、SSRF、XSS，各打一发确认面。
3. 有回显/差分再加深度；无回显先证外带（DNSLog/回连）。
4. 被 WAF 拦了再开 `kb:waf-bypass`，换编码/位置，不在开场硬刚。

## 判定
- 注入：数据库报错/布尔差分/时延/外带成功，且可重复。
- SSRF：DNSLog 或回连证明服务端发起了请求；元数据能读到才升级。
- XSS：在目标上下文执行并留下可复现证据；SRC 证明方式优先。
- RCE：命令/文件操作有可复现输出，不靠版本猜测。

## 常见坑
- 每个 path 喷 `'` 当 WAF 检测。
- 没有差分面硬打，制造空窗。
- 批量/高并发，触发 WAF 或影响业务。
- 把版本/CVE 推测当已确认漏洞。

## 主源
- `kb:injection-test`：§SQL 注入、§快速检测、§SSTI
- `kb:ssrf-test`：§检测 Payload、§云服务 Metadata
- `kb:xss-test`：§XSS 证明方式（SRC 要求）
- `skill:offensive-rce`

## 次源
- `skill:offensive-sqli`
- `skill:offensive-ssrf`
- `skill:offensive-xss`
- `skill:offensive-ssti`

## 打开
```bash
python3 scripts/index.py show kb:injection-test --section "SQL 注入"
python3 scripts/index.py show kb:ssrf-test --section "检测 Payload"
python3 scripts/index.py show kb:xss-test --section "XSS 证明方式"
```
