# 红队 / SRC 技能总索引

> 自动生成，勿手改。更新命令：`python3 scripts/index.py update`。

- 生成时间：2026-11-21 16:39
- 条目总数：147（技能 86 / SRC 入口 1 / 知识库 48 / 规则 12）
- 库根目录：`/app/skills/red-skill/library`

## 常用场景路由

| 场景 | 主源 | 次源 | 卡片 |
|---|---|---|---|
| 新目标 / 只给集团名 / 进站 | `src:entry` `rule:dig-scope-workflow` | `rule:src-value-hunting` `rule:hunt-iter` `kb:recon-methodology` | `cards/src-entry.md` |
| 资产测绘 / FOFA / 子域 / 种子队列 | `kb:recon-methodology` `rule:dig-scope-workflow` | `skill:bbot-recon` `skill:offensive-osint` `skill:offensive-osint-methodology` | `cards/recon-seed-loop.md` |
| 有登录态 / 越权 / 认证绕过 | `kb:idor-test` `kb:authbypass-test` | `kb:oauth-jwt-test` `skill:offensive-idor` `skill:api-security` | `cards/auth-object-graph.md` |
| 注入 / SQL / 命令 / 模板 | `kb:injection-test` | `skill:offensive-sqli` `skill:offensive-ssti` `skill:offensive-rce` | `cards/web-four.md` |
| SSRF / 服务端请求 / 元数据 | `kb:ssrf-test` | `skill:offensive-ssrf` | `cards/web-four.md` |
| XSS / 富文本 / 前端脚本 | `kb:xss-test` | `skill:offensive-xss` | `cards/web-four.md` |
| 文件上传 / 下载 / 路径穿越 | `kb:file-upload-test` `kb:path-traversal-lfi-test` | `skill:offensive-file-upload` | `cards/upload-path.md` |
| 反序列化 / XXE / JNDI | `kb:deserialization-test` `kb:jndi-injection-test` `kb:xxe-test` | `skill:offensive-deserialization` `skill:offensive-xxe` | `cards/upload-path.md` |
| API / GraphQL / JWT / OAuth | `kb:graphql-test` `kb:oauth-jwt-test` `kb:api-gateway-test` | `skill:api-security` `skill:offensive-api-security` `skill:offensive-graphql` `skill:offensive-jwt` `skill:offensive-oauth` `skill:offensive-api-abuse` | `cards/api-graphql.md` |
| 业务逻辑 / 支付 / 竞态 | `kb:logic-test` `kb:race-condition-test` | `skill:offensive-business-logic` `skill:offensive-race-condition` | `cards/logic-race.md` |
| 请求走私 / 缓存 / Host 头 | `kb:http-smuggling-test` `kb:cache-poisoning-test` `kb:http-host-header-test` | `skill:offensive-request-smuggling` `skill:offensive-parameter-pollution` |  |
| WAF 绕过 / JS 逆向 / 隐藏接口 | `kb:waf-bypass` `kb:js-reverse-guide` | `skill:offensive-waf-bypass` `rule:playwright-browser-mcp` | `cards/waf-js.md` |
| 云 IDE / Codex / 编程台 | `kb:cloud-ide-codex-rce-chain` | `skill:offensive-cloud` `rule:dig-scope-workflow` | `cards/cloud-ide.md` |
| 内网 / AD / Kerberos | `skill:offensive-active-directory` `skill:offensive-netexec` | `skill:offensive-lateral-movement` `skill:offensive-windows-privesc` `skill:offensive-linux-privesc` `skill:offensive-persistence` | `cards/infra-ad-cloud.md` |
| 云 / K8s / 容器逃逸 | `skill:offensive-k8s-attacks` `skill:offensive-cloud` | `skill:offensive-container-escape` | `cards/infra-ad-cloud.md` |
| 二进制 / 漏洞利用 / Fuzz | `skill:offensive-basic-exploitation` `skill:offensive-crash-analysis` | `skill:offensive-fuzzing` `skill:offensive-mitigations` `skill:offensive-exploit-development` `skill:offensive-vuln-classes` `skill:rea` `skill:patch-diff` `skill:codeaudit` | `cards/exploit-dev.md` |
| 移动 / IoT / 无线 | `skill:offensive-mobile` `skill:offensive-iot` `skill:offensive-wifi` | `skill:offensive-wpa3-sae` `skill:offensive-bluetooth-ble` `skill:offensive-bluetooth-classic` `skill:offensive-wifi-recon` |  |
| AI / Agent / 工具执行 | `kb:agent-tool-exec-test` `skill:offensive-ai-security` | `kb:llm-security-test` |  |
| 报告 / 定级 / 复现 | `rule:vuln-report-format` `skill:offensive-reporting` | `kb:info-leak-test` | `cards/reporting.md` |
| 浏览器自动化 / 抓包 | `rule:playwright-browser-mcp` | `kb:js-reverse-guide` |  |
| CORS（只做范围判断） | `rule:cors-vuln-report-priority` | `kb:cors-test` |  |

## SRC 总入口（1）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `src:entry` | SRC 漏洞挖掘 + 白盒 0day 审计技能 — SRC 总入口：黑盒挖洞 + 白盒 0day；进站节奏、短表、红线与知识库目录 | 148 | SRC入口 | `00-src/SKILL.md` |

## SRC 中文知识库（48）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `kb:401-403-bypass` | 401-403-bypass（禁开磨登录 HTML） — 禁开磨登录 HTML（已收成一行）；业务 API 401 现场自己打 | 4 | 知识库 | `00-src/知识库/401-403-bypass.md` |
| `kb:api-gateway-test` | API 网关安全测试手册 — API 网关 | 504 | 知识库 | `00-src/知识库/api-gateway-test.md` |
| `kb:authbypass-test` | authbypass-authentication-flaws — 认证绕过（未登录改密/IDaaS + 短表指针；英文字典已砍） | 322 | 知识库 | `00-src/知识库/authbypass-test.md` |
| `kb:clickjacking-test` | clickjacking-test（缺头不写） — 缺头不写（已收成一行） | 4 | 知识库 | `00-src/知识库/clickjacking-test.md` |
| `kb:cors-test` | cors（仅技术资料 · SRC 禁用） — 不挖勿开（已收成一行） | 4 | 知识库 | `00-src/知识库/cors-test.md` |
| `kb:crlf-injection-test` | crlf-injection-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/crlf-injection-test.md` |
| `kb:csp-bypass-test` | csp-bypass-test（几乎不交） — 几乎不交（已收成一行）；XSS 走 xss-test.md | 4 | 知识库 | `00-src/知识库/csp-bypass-test.md` |
| `kb:csrf-test` | csrf — 专题知识（hack-skills 导入或融合） | 594 | 知识库 | `00-src/知识库/csrf-test.md` |
| `kb:csv-formula-injection-test` | csv-formula-injection-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/csv-formula-injection-test.md` |
| `kb:dangling-markup-test` | dangling-markup-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/dangling-markup-test.md` |
| `kb:dependency-confusion-test` | dependency-confusion-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/dependency-confusion-test.md` |
| `kb:deserialization-test` | deserialization — 专题知识（hack-skills 导入或融合） | 1218 | 知识库 | `00-src/知识库/deserialization-test.md` |
| `kb:dns-rebinding-test` | dns-rebinding-test（几乎不交） — 几乎不交（已收成一行）；SSRF 走 ssrf-test.md | 4 | 知识库 | `00-src/知识库/dns-rebinding-test.md` |
| `kb:email-header-injection-test` | email-header-injection-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/email-header-injection-test.md` |
| `kb:el-injection-test` | expression-language-injection — 专题知识（hack-skills 导入或融合） | 230 | 知识库 | `00-src/知识库/el-injection-test.md` |
| `kb:ghost-bits-cast-test` | ghost-bits-cast-attack — Ghost Bits 原理+常用字+公式；逐字节两套表已砍 | 744 | 知识库 | `00-src/知识库/ghost-bits-cast-test.md` |
| `kb:graphql-test` | GraphQL 安全测试手册 — GraphQL（原有+补充） | 520 | 知识库 | `00-src/知识库/graphql-test.md` |
| `kb:hpp-test` | hpp-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/hpp-test.md` |
| `kb:http-smuggling-test` | HTTP 请求走私测试手册 — 请求走私（原有+补充） | 1344 | 知识库 | `00-src/知识库/http-smuggling-test.md` |
| `kb:http-host-header-test` | http-host-header — 专题知识（hack-skills 导入或融合） | 324 | 知识库 | `00-src/知识库/http-host-header-test.md` |
| `kb:http2-attacks-test` | http2-attacks-test（几乎不交） — 几乎不交（已收成一行）；走私走 http-smuggling-test.md | 4 | 知识库 | `00-src/知识库/http2-attacks-test.md` |
| `kb:insecure-scm-test` | insecure-scm — 专题知识（hack-skills 导入或融合） | 160 | 知识库 | `00-src/知识库/insecure-scm-test.md` |
| `kb:jndi-injection-test` | jndi-injection — 专题知识（hack-skills 导入或融合） | 272 | 知识库 | `00-src/知识库/jndi-injection-test.md` |
| `kb:js-reverse-guide` | JS 逆向配合接口挖掘指南 — JS 逆向 | 201 | 知识库 | `00-src/知识库/js-reverse-guide.md` |
| `kb:llm-security-test` | llm-security-test（禁开越狱教材） — 禁开越狱教材（已收成一行）；对话工具走 agent-tool-exec-test.md | 4 | 知识库 | `00-src/知识库/llm-security-test.md` |
| `kb:oauth-jwt-test` | OAuth/JWT/SAML 安全测试手册 — OAuth/JWT/SAML/OIDC（原有+多源补充） | 835 | 知识库 | `00-src/知识库/oauth-jwt-test.md` |
| `kb:open-redirect-test` | open-redirect — 专题知识（hack-skills 导入或融合） | 380 | 知识库 | `00-src/知识库/open-redirect-test.md` |
| `kb:path-traversal-lfi-test` | path-traversal-lfi — 专题知识（hack-skills 导入或融合） | 825 | 知识库 | `00-src/知识库/path-traversal-lfi-test.md` |
| `kb:prototype-pollution-test` | prototype-pollution — 专题知识（hack-skills 导入或融合） | 774 | 知识库 | `00-src/知识库/prototype-pollution-test.md` |
| `kb:recon-methodology` | recon-and-methodology — 侦察方法论 | 412 | 知识库 | `00-src/知识库/recon-methodology.md` |
| `kb:ssrf-test` | SSRF 测试手册 — SSRF（IMDS 路径差 / GOPROXY / 对象存储回源） | 147 | 知识库 | `00-src/知识库/ssrf-test.md` |
| `kb:subdomain-takeover-test` | subdomain-takeover — 专题知识（hack-skills 导入或融合） | 231 | 知识库 | `00-src/知识库/subdomain-takeover-test.md` |
| `kb:type-juggling-test` | type-juggling — 专题知识（hack-skills 导入或融合） | 286 | 知识库 | `00-src/知识库/type-juggling-test.md` |
| `kb:waf-bypass` | waf-bypass — WAF 绕过 | 475 | 知识库 | `00-src/知识库/waf-bypass.md` |
| `kb:websocket-test` | WebSocket 安全测试手册 — WebSocket（原有+补充） | 1055 | 知识库 | `00-src/知识库/websocket-test.md` |
| `kb:xslt-injection-test` | xslt-injection-test（几乎不交） — 几乎不交（已收成一行） | 4 | 知识库 | `00-src/知识库/xslt-injection-test.md` |
| `kb:xss-test` | XSS 测试手册 — XSS（中文开场 + 冷门事件 + XSS→RCE / 自定义协议） | 246 | 知识库 | `00-src/知识库/xss-test.md` |
| `kb:xxe-test` | xxe — 专题知识（hack-skills 导入或融合） | 649 | 知识库 | `00-src/知识库/xxe-test.md` |
| `kb:cloud-ide-codex-rce-chain` | 云 IDE / Codex 系 AI 编程平台：弱口令→Root RCE→凭证链 — 云 IDE/Codex 系：弱口令→RPC RCE→集群/API Key 链（短表有指针） | 204 | 知识库 | `00-src/知识库/cloud-ide-codex-rce-chain.md` |
| `kb:info-leak-test` | 信息泄露测试手册 — 信息泄露 | 257 | 知识库 | `00-src/知识库/info-leak-test.md` |
| `kb:agent-tool-exec-test` | 对话口工具真执行 — 对话口工具真执行（不是越狱、不是云 IDE RPC） | 53 | 知识库 | `00-src/知识库/agent-tool-exec-test.md` |
| `kb:打穿短表` | 打穿短表（索引） — 挖洞手法索引（一行/指针；正文仍在各模块） | 95 | 知识库 | `00-src/知识库/打穿短表.md` |
| `kb:file-upload-test` | 文件上传漏洞测试手册 — 文件上传（STS/列桶/分享鉴权等指针） | 168 | 知识库 | `00-src/知识库/file-upload-test.md` |
| `kb:injection-test` | 注入类漏洞测试手册（SQL注入 / 命令注入 / SSTI） — 注入（OR+total / 邮件订阅 iframe / SSTI 探测；英文百科已砍） | 80 | 知识库 | `00-src/知识库/injection-test.md` |
| `kb:race-condition-test` | 竞态条件测试手册 — 竞态（原有+补充） | 1070 | 知识库 | `00-src/知识库/race-condition-test.md` |
| `kb:cache-poisoning-test` | 缓存投毒测试手册 — 缓存投毒/欺骗（原有+补充） | 1299 | 知识库 | `00-src/知识库/cache-poisoning-test.md` |
| `kb:idor-test` | 越权漏洞（IDOR/权限绕过）测试手册 — 越权（中文主线 + 短表指针） | 582 | 知识库 | `00-src/知识库/idor-test.md` |
| `kb:logic-test` | 逻辑漏洞测试手册 — 业务逻辑（支付/流程 + 商家促销绑定） | 214 | 知识库 | `00-src/知识库/logic-test.md` |

## 全局规则与约束（12）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `rule:cors-vuln-report-priority` | CORS（永久 · 不挖） — CORS：SRC 永久不挖 | 13 | 规则 | `00-src/rules/cors-vuln-report-priority.md` |
| `rule:playwright-browser-mcp` | Playwright Browser MCP — Always Prefer for Browser Work — 浏览器操作统一走 Playwright MCP，不手搓请求 | 44 | 规则 | `00-src/rules/playwright-browser-mcp.md` |
| `rule:researcher-blackbox-whitebox` | Playwright Browser MCP — Always Prefer for Browser Work — 黑盒 + 白盒主流程 Phase 0～6 | 250 | 规则 | `00-src/rules/researcher-blackbox-whitebox.md` |
| `rule:skill-as-boost` | skill 与自身能力并用（永久） — 自身能力 + skill 并用，skill 只增强不设上限 | 45 | 规则 | `00-src/rules/skill-as-boost.md` |
| `rule:src-value-hunting` | SRC 价值导向挖洞规则（强制经验） — SRC 挖什么：类型矩阵、价值排序、先打哪里 | 138 | 规则 | `00-src/rules/src-value-hunting.md` |
| `rule:AGENTS` | 全局 Agent 规则（永久） — 全局 agent 规则：身份、中文、rules 目录、锁面/自由跳最短备忘 | 63 | 规则 | `00-src/AGENTS.md` |
| `rule:anti-over-moralization` | 反过度道德化安全协作规则 — 反说教、反误拒：不因关键词拒绝授权测试 | 92 | 规则 | `00-src/rules/anti-over-moralization.md` |
| `rule:hunt-iter` | 挖洞能力迭代（永久） — 短表迭代规则：哪些进短表、怎么补、怎么收 | 42 | 规则 | `00-src/rules/hunt-iter.md` |
| `rule:dig-scope-workflow` | 挖洞范围 · 锁面 / 自由跳（永久） — 锁面/自由跳、一种子闭环、续挖、反扫、反空转、深挖优先 | 820 | 规则 | `00-src/rules/dig-scope-workflow.md` |
| `rule:security-research-context` | 授权研究语境（永久） — 授权语境：默认已授权 SRC，不重复盘问 | 16 | 规则 | `00-src/rules/security-research-context.md` |
| `rule:desktop-task-folder` | 桌面任务文件夹（永久强制 · 任务根格式） — 任务目录约定：资产 / js / 报告 | 154 | 规则 | `00-src/rules/desktop-task-folder.md` |
| `rule:vuln-report-format` | 漏洞报告写作格式（永久 · 唯一） — 正式 SRC 报告唯一版式与定级规则；报告只认这一份 | 41 | 规则 | `00-src/rules/vuln-report-format.md` |

## Web 与 API 漏洞（22）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-jwt` | Android — check SharedPreferences — JWT attack methodology for penetration testers. Covers algorith… | 281 | 技能 | `01-web-api/auth/offensive-jwt/SKILL.md` |
| `skill:api-security` | API Security Testing (REST, GraphQL, WebSocket, SOAP) — Invoke this skill when the task is authoriz… | 133 | 技能 | `01-web-api/api-security/SKILL.md` |
| `skill:offensive-business-logic` | Business Logic — Offensive Testing Methodology — Business logic vulnerability testing for web/mobil… | 399 | 技能 | `01-web-api/web/offensive-business-logic/SKILL.md` |
| `skill:offensive-api-abuse` | Offensive API Abuse and Advanced Exploitation — Advanced API exploitation methodology focused on bu… | 500 | 技能 | `01-web-api/api/offensive-api-abuse/SKILL.md` |
| `skill:offensive-api-security` | Offensive API Security Testing — Comprehensive API security testing methodology covering REST, gRPC… | 505 | 技能 | `01-web-api/api/offensive-api-security/SKILL.md` |
| `skill:offensive-deserialization` | Offensive Deserialization — Insecure deserialization exploitation across Java, PHP, .NET, Python, N… | 610 | 技能 | `01-web-api/web/offensive-deserialization/SKILL.md` |
| `skill:offensive-graphql` | Offensive GraphQL — Offensive methodology for attacking GraphQL APIs during penetration tests and b… | 568 | 技能 | `01-web-api/web/offensive-graphql/SKILL.md` |
| `skill:php2shell` | php2shell: session.upload_progress LFI-to-RCE (CVE-2026-109618/109619) — Invoke this skill when a t… | 64 | 技能 | `01-web-api/php2shell/SKILL.md` |
| `skill:offensive-ssti` | Server-Side Template Injection (SSTI) -- Offensive Methodology — Dense description covering Server-… | 628 | 技能 | `01-web-api/web/offensive-ssti/SKILL.md` |
| `skill:offensive-xss` | SKILL: Cross-Site Scripting (XSS) — Cross-Site Scripting testing checklist: stored/reflected/DOM/bl… | 788 | 技能 | `01-web-api/web/offensive-xss/SKILL.md` |
| `skill:offensive-file-upload` | SKILL: File Upload Vulnerabilities — File upload vulnerability checklist: MIME type bypass, extensi… | 826 | 技能 | `01-web-api/web/offensive-file-upload/SKILL.md` |
| `skill:offensive-parameter-pollution` | SKILL: HTTP Parameter Pollution (HPP) — HTTP parameter pollution (HPP) checklist: duplicate paramet… | 599 | 技能 | `01-web-api/web/offensive-parameter-pollution/SKILL.md` |
| `skill:offensive-request-smuggling` | SKILL: HTTP Request Smuggling — HTTP request smuggling checklist: CL.TE, TE.CL, TE.TE variants, det… | 777 | 技能 | `01-web-api/web/offensive-request-smuggling/SKILL.md` |
| `skill:offensive-idor` | SKILL: Insecure Direct Object References (IDOR) — IDOR (Insecure Direct Object Reference) testing c… | 612 | 技能 | `01-web-api/web/offensive-idor/SKILL.md` |
| `skill:offensive-oauth` | SKILL: OAuth Security Testing — OAuth 2.0 attack checklist: authorization code interception, redire… | 370 | 技能 | `01-web-api/auth/offensive-oauth/SKILL.md` |
| `skill:offensive-open-redirect` | SKILL: Open Redirect Vulnerabilities — Open redirect vulnerability checklist: parameter identificat… | 491 | 技能 | `01-web-api/web/offensive-open-redirect/SKILL.md` |
| `skill:offensive-race-condition` | SKILL: Race Conditions — Race condition (TOCTOU) testing checklist: identifying timing windows, Bur… | 885 | 技能 | `01-web-api/web/offensive-race-condition/SKILL.md` |
| `skill:offensive-rce` | SKILL: Remote Code Execution — Remote Code Execution testing checklist: OS command injection, SSTI-… | 1073 | 技能 | `01-web-api/web/offensive-rce/SKILL.md` |
| `skill:offensive-ssrf` | SKILL: Server-Side Request Forgery (SSRF) — Server-Side Request Forgery testing checklist: SSRF dis… | 834 | 技能 | `01-web-api/web/offensive-ssrf/SKILL.md` |
| `skill:offensive-waf-bypass` | SKILL: WAF Bypass Techniques — WAF bypass techniques checklist: encoding bypass (URL/HTML/Unicode/d… | 827 | 技能 | `01-web-api/web/offensive-waf-bypass/SKILL.md` |
| `skill:offensive-xxe` | SKILL: XML External Entity (XXE) Injection — XML External Entity injection testing checklist: class… | 1000 | 技能 | `01-web-api/web/offensive-xxe/SKILL.md` |
| `skill:offensive-sqli` | SQL Injection — Offensive Testing Methodology — SQL injection testing skill for offensive security… | 371 | 技能 | `01-web-api/web/offensive-sqli/SKILL.md` |

## 侦察与资产测绘（3）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:bbot-recon` | bbot-recon: recursive recon under program-policy gates — Invoke this skill when a hunt task needs b… | 133 | 技能 | `02-recon/bbot-recon/SKILL.md` |
| `skill:offensive-osint` | Offensive OSINT Methodology — Comprehensive OSINT methodology skill for offensive security, red tea… | 400 | 技能 | `02-recon/recon/offensive-osint/SKILL.md` |
| `skill:offensive-osint-methodology` | SKILL: OSINT Methodology — Structured OSINT methodology framework: target definition, source select… | 440 | 技能 | `02-recon/recon/offensive-osint-methodology/SKILL.md` |

## 云与容器（3）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-cloud` | Cloud (AWS / Azure / GCP) — Offensive Testing Methodology — Cloud security attack methodology cover… | 347 | 技能 | `03-cloud-container/cloud/offensive-cloud/SKILL.md` |
| `skill:offensive-container-escape` | Container Escape and Breakout — Container escape and breakout techniques targeting Docker, containe… | 673 | 技能 | `03-cloud-container/container/offensive-container-escape/SKILL.md` |
| `skill:offensive-k8s-attacks` | Kubernetes Cluster Attacks — Kubernetes cluster attack techniques covering the full attack lifecycl… | 809 | 技能 | `03-cloud-container/container/offensive-k8s-attacks/SKILL.md` |

## 内网、AD 与红队基础设施（15）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-active-directory` | Active Directory — Offensive Testing Methodology — Active Directory attack methodology for internal… | 429 | 技能 | `04-internal/active-directory/offensive-active-directory/SKILL.md` |
| `skill:offensive-advanced-redteam` | Advanced Red Team Operations — Comprehensive red team operations methodology covering full engageme… | 641 | 技能 | `04-internal/infrastructure/offensive-advanced-redteam/SKILL.md` |
| `skill:offensive-data-exfiltration` | Data Exfiltration -- Offensive Methodology — Dense methodology covering DNS exfiltration (dnscat2,… | 551 | 技能 | `04-internal/post-exploitation/offensive-data-exfiltration/SKILL.md` |
| `skill:offensive-linux-privesc` | Linux Privilege Escalation — Comprehensive Linux privilege escalation methodology for offensive sec… | 585 | 技能 | `04-internal/privesc/offensive-linux-privesc/SKILL.md` |
| `skill:offensive-shellcode` | Locate kernel32 Base Address — Shellcode development reference for offensive security engagements.… | 476 | 技能 | `04-internal/infrastructure/offensive-shellcode/SKILL.md` |
| `skill:offensive-netexec` | NetExec (nxc) — Reference Skill — Use this skill whenever the user asks about NetExec (nxc) — a net… | 856 | 技能 | `04-internal/active-directory/offensive-netexec/SKILL.md` |
| `skill:offensive-network-attacks` | Network Attacks (Layer 2/3) -- Offensive Methodology — Dense description covering ARP spoofing, LLM… | 492 | 技能 | `04-internal/network/offensive-network-attacks/SKILL.md` |
| `skill:offensive-lateral-movement` | Offensive Lateral Movement — Comprehensive lateral movement tradecraft for authorized red team enga… | 451 | 技能 | `04-internal/post-exploitation/offensive-lateral-movement/SKILL.md` |
| `skill:offensive-persistence` | Offensive Persistence — Comprehensive persistence tradecraft for authorized red team engagements co… | 601 | 技能 | `04-internal/post-exploitation/offensive-persistence/SKILL.md` |
| `skill:offensive-edr-evasion` | SKILL: Endpoint Detection and Response — EDR evasion offensive checklist: hook unhooking (user/kern… | 1810 | 技能 | `04-internal/infrastructure/offensive-edr-evasion/SKILL.md` |
| `skill:offensive-initial-access` | SKILL: Modern Initial Access — Initial access techniques checklist: phishing (spear/smishing), cred… | 1532 | 技能 | `04-internal/infrastructure/offensive-initial-access/SKILL.md` |
| `skill:offensive-keylogger-arch` | SKILL: Novel research — Low-level keylogger architecture design: kernel driver hooks (WH_KEYBOARD_L… | 201 | 技能 | `04-internal/infrastructure/offensive-keylogger-arch/SKILL.md` |
| `skill:offensive-windows-mitigations` | SKILL: Week 6: Understanding Windows Mitigations — Deep-dive on Windows exploit mitigations: ASLR,… | 14550 | 技能 | `04-internal/infrastructure/offensive-windows-mitigations/SKILL.md` |
| `skill:offensive-windows-boundaries` | SKILL: Week 7: Defeating Windows Security Boundaries — Windows security boundary taxonomy and attac… | 15157 | 技能 | `04-internal/infrastructure/offensive-windows-boundaries/SKILL.md` |
| `skill:offensive-windows-privesc` | Windows Privilege Escalation — Comprehensive Windows privilege escalation methodology for offensive… | 581 | 技能 | `04-internal/privesc/offensive-windows-privesc/SKILL.md` |

## 移动、IoT 与无线（16）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-bluetooth-classic` | Bluetooth Classic (BR/EDR) Attacks — Bluetooth Classic (BR/EDR) attack methodology — device discove… | 157 | 技能 | `05-mobile-iot-wireless/wireless/offensive-bluetooth-classic/SKILL.md` |
| `skill:offensive-bluetooth-ble` | Bluetooth Low Energy (BLE) Attacks — Bluetooth Low Energy (BLE) attack methodology — GATT enumerati… | 195 | 技能 | `05-mobile-iot-wireless/wireless/offensive-bluetooth-ble/SKILL.md` |
| `skill:offensive-deauth-disassoc` | Deauth / Disassoc Attacks — Deauthentication and disassociation attacks against 802.11 networks — t… | 133 | 技能 | `05-mobile-iot-wireless/wireless/offensive-deauth-disassoc/SKILL.md` |
| `skill:offensive-evil-twin` | Evil Twin / KARMA / Mana — Evil Twin / KARMA / Mana access point methodology — rogue AP constructio… | 221 | 技能 | `05-mobile-iot-wireless/wireless/offensive-evil-twin/SKILL.md` |
| `skill:offensive-iot` | IoT & Embedded — Offensive Testing Methodology — IoT and embedded device security testing methodolo… | 436 | 技能 | `05-mobile-iot-wireless/iot/offensive-iot/SKILL.md` |
| `skill:offensive-krack-fragattacks` | KRACK & FragAttacks — KRACK (CVE-2017-13077..082) and FragAttacks (CVE-2020-24586..588 + 26139-2614… | 115 | 技能 | `05-mobile-iot-wireless/wireless/offensive-krack-fragattacks/SKILL.md` |
| `skill:offensive-lorawan-sub-ghz` | LoRaWAN & Sub-GHz Attacks — LoRaWAN and sub-GHz (433 / 868 / 915 MHz) attack methodology — LoRaWAN… | 183 | 技能 | `05-mobile-iot-wireless/wireless/offensive-lorawan-sub-ghz/SKILL.md` |
| `skill:offensive-mobile` | Mobile (Android + iOS) — Offensive Testing Methodology — Mobile (Android + iOS) application penetra… | 423 | 技能 | `05-mobile-iot-wireless/mobile/offensive-mobile/SKILL.md` |
| `skill:offensive-wifi-recon` | Wi-Fi Reconnaissance — Wi-Fi reconnaissance methodology — adapter selection, monitor mode and packe… | 204 | 技能 | `05-mobile-iot-wireless/wireless/offensive-wifi-recon/SKILL.md` |
| `skill:offensive-wifi` | Wireless / 802.11 — Offensive Testing Methodology — Wireless / 802.11 attack methodology for red te… | 346 | 技能 | `05-mobile-iot-wireless/wireless/offensive-wifi/SKILL.md` |
| `skill:offensive-wpa-enterprise` | WPA-Enterprise (802.1X / EAP) Attacks — WPA/WPA2/WPA3-Enterprise (802.1X / EAP) attack methodology… | 208 | 技能 | `05-mobile-iot-wireless/wireless/offensive-wpa-enterprise/SKILL.md` |
| `skill:offensive-wpa2-psk` | WPA/WPA2-PSK Attacks — WPA/WPA2-PSK attack methodology — four-way handshake capture via targeted de… | 177 | 技能 | `05-mobile-iot-wireless/wireless/offensive-wpa2-psk/SKILL.md` |
| `skill:offensive-wpa3-sae` | WPA3 / SAE Attacks — WPA3 / SAE (Simultaneous Authentication of Equals) attack methodology — transi… | 160 | 技能 | `05-mobile-iot-wireless/wireless/offensive-wpa3-sae/SKILL.md` |
| `skill:offensive-wps` | WPS PIN Attacks — WPS (Wi-Fi Protected Setup) PIN attack methodology — Pixie Dust offline attack ag… | 167 | 技能 | `05-mobile-iot-wireless/wireless/offensive-wps/SKILL.md` |
| `skill:offensive-z-wave` | Z-Wave Attacks — Z-Wave attack methodology — sniffing with Z-Force / EZ-Wave / RTL-SDR + ZniffMobil… | 147 | 技能 | `05-mobile-iot-wireless/wireless/offensive-z-wave/SKILL.md` |
| `skill:offensive-zigbee-thread-matter` | Zigbee / Thread / Matter Attacks — Zigbee, Thread, and Matter mesh-protocol attack methodology — IE… | 169 | 技能 | `05-mobile-iot-wireless/wireless/offensive-zigbee-thread-matter/SKILL.md` |

## 二进制、逆向与代码审计（14）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:codeaudit` | Code Audit (white-box source review) — Invoke this skill when the task is a white-box audit of sour… | 75 | 技能 | `06-binary/codeaudit/SKILL.md` |
| `skill:edr-bypass-re` | EDR Bypass RE (defense telemetry analysis for authorized adversary simulation) — Invoke this skill… | 101 | 技能 | `06-binary/edr-bypass-re/SKILL.md` |
| `skill:offensive-fuzzing` | Offensive Fuzzing — Practical offensive fuzzing methodology covering target identification, fuzzer… | 339 | 技能 | `06-binary/fuzzing/offensive-fuzzing/SKILL.md` |
| `skill:patch-diff` | Patch Diff (N-day recovery from vendor fixes) — Invoke this skill when the task is recovering a vul… | 111 | 技能 | `06-binary/patch-diff/SKILL.md` |
| `skill:rea` | rea: binary and application reverse engineering — Reverse-engineer binaries and application package… | 40 | 技能 | `06-binary/rea/SKILL.md` |
| `skill:offensive-bug-identification` | SKILL: Bug Identification — Systematic bug identification methodology: source code review patterns,… | 1260 | 技能 | `06-binary/fuzzing/offensive-bug-identification/SKILL.md` |
| `skill:offensive-exploit-dev-course` | SKILL: Exploit Development — Full exploit development course roadmap and syllabus: weekly topics, r… | 432 | 技能 | `06-binary/exploit-dev/offensive-exploit-dev-course/SKILL.md` |
| `skill:offensive-exploit-development` | SKILL: Exploit Development — Exploit development operational guide: environment setup, debugging wo… | 703 | 技能 | `06-binary/exploit-dev/offensive-exploit-development/SKILL.md` |
| `skill:offensive-mitigations` | SKILL: Modern Kernel Exploit Mitigations — Security mitigation reference and bypass catalog: ASLR,… | 1355 | 技能 | `06-binary/exploit-dev/offensive-mitigations/SKILL.md` |
| `skill:offensive-vuln-classes` | SKILL: Week 1: Vulnerability Classes with Real-World Examples — Exploit development curriculum cove… | 1233 | 技能 | `06-binary/fuzzing/offensive-vuln-classes/SKILL.md` |
| `skill:offensive-fuzzing-course` | SKILL: Week 2: Finding Vulnerabilities Through Fuzzing — Week 2 of the exploit development curricul… | 2109 | 技能 | `06-binary/fuzzing/offensive-fuzzing-course/SKILL.md` |
| `skill:offensive-crash-analysis` | SKILL: Week 4: Crash Analysis and Exploitability Assessment — Week 4 exploit development curriculum… | 12470 | 技能 | `06-binary/exploit-dev/offensive-crash-analysis/SKILL.md` |
| `skill:offensive-basic-exploitation` | SKILL: Week 5: Basic Exploitation (Linux with Mitigations Disabled) — Week 5 exploit development cu… | 10787 | 技能 | `06-binary/exploit-dev/offensive-basic-exploitation/SKILL.md` |
| `skill:offensive-toctou` | TOCTOU — Time-of-Check / Time-of-Use Exploitation — Time-of-Check / Time-of-Use (TOCTOU) race condi… | 348 | 技能 | `06-binary/exploit-dev/offensive-toctou/SKILL.md` |

## 供应链与 CI/CD（4）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-cicd-pipeline` | Offensive CI/CD Pipeline Exploitation — Comprehensive CI/CD pipeline exploitation methodology cover… | 520 | 技能 | `07-supply-cicd/cicd/offensive-cicd-pipeline/SKILL.md` |
| `skill:offensive-cicd-secrets` | Offensive CI/CD Secrets Extraction — Comprehensive secrets extraction methodology targeting CI/CD e… | 580 | 技能 | `07-supply-cicd/cicd/offensive-cicd-secrets/SKILL.md` |
| `skill:offensive-dependency-confusion` | Offensive Dependency Confusion and Namespace Attacks — Deep-dive offensive methodology for dependen… | 603 | 技能 | `07-supply-cicd/supply-chain/offensive-dependency-confusion/SKILL.md` |
| `skill:offensive-supply-chain` | Offensive Supply Chain Attacks — Comprehensive offensive methodology for software supply chain atta… | 547 | 技能 | `07-supply-cicd/supply-chain/offensive-supply-chain/SKILL.md` |

## 社工与钓鱼（2）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-phishing` | Offensive Phishing — Phishing campaign execution methodology for authorized red team engagements. C… | 520 | 技能 | `08-social/social-engineering/offensive-phishing/SKILL.md` |
| `skill:offensive-social-engineering` | Offensive Social Engineering — Social engineering attack techniques beyond email phishing for autho… | 577 | 技能 | `08-social/social-engineering/offensive-social-engineering/SKILL.md` |

## 取证、反取证与 C2（2）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-anti-forensics` | Offensive Anti-Forensics — Anti-forensics and evidence destruction techniques for red team operator… | 468 | 技能 | `09-forensics/forensics/offensive-anti-forensics/SKILL.md` |
| `skill:offensive-c2-frameworks` | Offensive C2 Frameworks — Command and Control framework deployment, configuration, and operational… | 597 | 技能 | `09-forensics/forensics/offensive-c2-frameworks/SKILL.md` |

## 密码学与 TLS（2）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-crypto-attacks` | Cryptographic Implementation Attacks — Systematic methodology for identifying and exploiting crypto… | 477 | 技能 | `10-crypto/crypto/offensive-crypto-attacks/SKILL.md` |
| `skill:offensive-tls-attacks` | TLS/SSL Attacks and Misconfiguration Exploitation — Comprehensive methodology for auditing and expl… | 536 | 技能 | `10-crypto/crypto/offensive-tls-attacks/SKILL.md` |

## AI 与 Agent 安全（1）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-ai-security` | SKILL: AI Pentest — AI/LLM security offensive checklist: prompt injection, jailbreaking, model extr… | 596 | 技能 | `11-ai/ai/offensive-ai-security/SKILL.md` |

## 工具与报告（2）

| ID | 摘要 | 行数 | 类型 | 路径 |
|---|---|---:|---|---|
| `skill:offensive-reporting` | Penetration Test Reporting — Professional Methodology — Penetration test and red team report writin… | 372 | 技能 | `12-utility/utility/offensive-reporting/SKILL.md` |
| `skill:offensive-fast-checking` | SKILL: Fast Testing Checklist — Speed-optimized offensive checklist for rapid assessment: quick-win… | 491 | 技能 | `12-utility/utility/offensive-fast-checking/SKILL.md` |
