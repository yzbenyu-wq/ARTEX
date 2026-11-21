# 决策卡：API / GraphQL / JWT / OAuth

## 何时用
- API 网关、微服务、前后端分离。
- GraphQL、WebSocket、JWT、OAuth/OIDC/SAML。
- 接口返回字段多、有隐藏字段。

## 首打 5 步
1. 识别端点：`/graphql`、`/api`、ws、swagger/openapi、前端 JS 里的 baseURL。
2. GraphQL 先试 Introspection；拿 schema 后再测越权与注入。
3. 越权：水平（同角色换 id）+ 垂直（低权限打高权限）；两账号证边界。
4. JWT：看算法、kid、密钥、声明信任；只做验证，不伪造他人身份长期使用。
5. OAuth/OIDC：看 redirect_uri、state、code 复用、scope 提升。

## 判定
- GraphQL：schema 泄露、跨用户字段读取、注入回显/差分。
- API：BOLA/BFLA 成立，能指出具体对象与角色差异。
- JWT/OAuth：能证明令牌/回调校验缺陷，且有可复现步骤。

## 常见坑
- 只扫端点不测对象级授权。
- Introspection 关了就直接放弃；还应测字段猜测/错误回显。
- 高频请求触发限流；并发只用少量。
- 登出/吊销令牌（禁止）。

## 主源
- `kb:graphql-test`：§GraphQL 识别、§Introspection、§越权测试
- `kb:oauth-jwt-test`
- `kb:api-gateway-test`
- `kb:websocket-test`

## 次源
- `skill:api-security`
- `skill:offensive-graphql`
- `skill:offensive-api-security`
- `skill:offensive-jwt`
- `skill:offensive-oauth`

## 打开
```bash
python3 scripts/index.py show kb:graphql-test --section "Introspection"
python3 scripts/index.py show kb:graphql-test --section "越权测试"
python3 scripts/index.py show skill:api-security
```
