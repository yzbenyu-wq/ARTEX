# 决策卡：登录态 / 越权 / 认证

## 何时用
- 目标有用户体系、登录态、对象 id、租户 id。
- 想测 IDOR/BOLA/BFLA、未授权访问、任意登录/接管。
- 用户给了 Cookie/Token。

## 首打 5 步
1. 建 A/B：两个账号最好；没有就“已登录 + 未登录”两组。
2. 找标识：用户 id、对象 id、租户/组织 id、隐藏字段、返回体里的 owner。
3. 先读差分：GET/列表/详情换 id，证明跨用户/跨租户可读。
4. 写越权最小伤害：先添加自己的 → 看能否挂到别人名下 → 删自己刚加的那条。
5. 改密/改绑/接管可探；过了立刻改回；改不回就停在回包。

## 判定
- 回包出现别人的对象/字段，或写操作改变了别人的归属。
- 只读红线 ≠ 写越权不用测；写越权要测，但禁止改删别人已有对象。
- 越权成立要能指出“哪个 id 换成哪个 id、谁的资源”。

## 硬红线
- 禁止登出/注销/退出/吊销令牌；全程保持用户会话有效。
- 禁止改/删别人已存在的订单、地址、密码、角色、邮箱。
- 扣钱、清库存、批量操作不做。
- CORS 不挖。

## 常见坑
- 只测 GET，不测写口。
- 把未授权 401 当没洞；先分清登录页、认证口、业务 API。
- 拿一个账号自测自，没证明边界。
- 改完别人的数据不恢复。

## 主源
- `kb:idor-test`：§测试流程、§写越权怎么打（先加后清）
- `kb:authbypass-test`：未登录改密口、IDaaS、联合登录

## 次源
- `kb:oauth-jwt-test`
- `skill:offensive-idor`
- `skill:api-security`

## 打开
```bash
python3 scripts/index.py show kb:idor-test --section "测试流程"
python3 scripts/index.py show kb:idor-test --section "写越权怎么打"
python3 scripts/index.py show kb:authbypass-test --section "未登录改密口"
```
