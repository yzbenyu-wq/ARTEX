# 决策卡：云 IDE / Codex / 编程台

## 何时用
- 目标是云 IDE、在线编程台、Codex 系工作区、admin 控制台。
- 看到 `/codex-api/rpc`、工作区容器、文件树、终端入口。
- 有登录页但业务面在 RPC/API。

## 首打 5 步
1. 指纹：页面标题、静态资源、`/codex-api`、WebSocket、工作区路径。
2. 登录面：只试少量默认口令/已知弱口令，不批量。
3. 登录后看 RPC/文件/元方法：工作区列表、文件读写、命令执行入口。
4. 证明到“可读授权测试文件 / 可执行一条无害命令”即停。
5. 找同构变体：同厂商、同路径、同静态资源，不要只会一个默认口令。

## 判定
- 未授权进入工作区、读到文件、执行命令，三者任一有可复现证据。
- 危害证明要含：入口、请求、回包、时间、身份。
- 不靠页面标题猜；必须有实际响应。

## 常见坑
- 批量试口令，触发封禁。
- 在工作区留后门/改配置。
- 动别人的工作区或项目。
- 把“能打开页面”当 RCE。

## 主源
- `kb:cloud-ide-codex-rce-chain`：§模式画像、§最小探测矩阵、§危害证明、§资产怎么找、§同构变体

## 次源
- `skill:offensive-cloud`
- `rule:dig-scope-workflow`（资产与范围）

## 打开
```bash
python3 scripts/index.py show kb:cloud-ide-codex-rce-chain --section "模式画像"
python3 scripts/index.py show kb:cloud-ide-codex-rce-chain --section "最小探测矩阵"
python3 scripts/index.py show kb:cloud-ide-codex-rce-chain --section "危害证明"
```
