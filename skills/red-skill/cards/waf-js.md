# 决策卡：WAF 绕过 / JS 逆向

## 何时用
- 参数被 WAF 拦、回 403/406/自定义拦截页。
- 前端有加密参数、签名、时间戳、nonce。
- 需要找隐藏接口、演示号、内部 path。

## 首打 5 步
1. 先指纹：状态码、拦截页、Header、触发规则，确认是哪类 WAF。
2. 换编码：URL/Unicode/HTML/Base64/大小写/注释；换位置：Header/Cookie/Body。
3. 换协议形态：分块、HTTP/2、multipart boundary、HPP、路径归一化、Content-Type。
4. JS 逆向：先批量抽接口，再断点找签名函数，最后在浏览器执行签名。
5. 只用一个被拦请求做 A/B，证明“原请求被拦、变形后业务正常”。

## 判定
- 变形请求通过 WAF 且业务回包正常（不是拦截页）。
- 签名可复现：同参数同时间窗能生成可用签名。
- 隐藏接口能返回业务数据。

## 常见坑
- 开场对每个 path 喷 `'` 当 WAF 检测。
- 硬刚规则到 IP 被封；应换池/降速。
- 手搓签名，忽略浏览器里的环境依赖。
- 用裸 curl 默认头，暴露工具指纹。

## 主源
- `kb:waf-bypass`：§PHASE 0、§GENERIC BYPASS
- `kb:js-reverse-guide`：§接口发现、§加密参数分析

## 次源
- `skill:offensive-waf-bypass`
- `rule:playwright-browser-mcp`（浏览器交互统一走 MCP）

## 打开
```bash
python3 scripts/index.py show kb:waf-bypass --section "PHASE 0"
python3 scripts/index.py show kb:waf-bypass --section "GENERIC BYPASS"
python3 scripts/index.py show kb:js-reverse-guide --section "接口发现"
python3 scripts/index.py show kb:js-reverse-guide --section "加密参数分析"
```
