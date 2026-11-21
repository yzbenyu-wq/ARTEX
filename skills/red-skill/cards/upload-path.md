# 决策卡：上传 / 路径 / XXE / 反序列化

## 何时用
- 有文件上传、下载、读文件、预览、导入导出。
- XML/SOAP/Office 解析、Java 中间件、对象存储。

## 首打 5 步
1. 上传先看：STS 临时凭证、对象 key 是否通配、签名是否绑 Host/Content-Type、桶策略、分享鉴权。
2. 下载/读文件：先测路径归一化与编码绕过，证明能读到目标文件。
3. XXE：先测外部实体是否解析，能外带再升级。
4. 反序列化：认 Java/Fastjson/Log4j/JNDI，先证外带/回连。
5. 只做最小证明：读到指定文件、列出桶、执行一条无害命令即停。

## 判定
- 上传：能覆盖/写入非预期 key，或拿到可访问的持久 URL。
- 路径：能读到授权测试文件（不是系统敏感文件）。
- XXE：外部实体解析成功并有外带证据。
- 反序列化：可复现的外带/回连，不靠报错猜。

## 常见坑
- 把 webshell 传到生产并留在那里。
- 批量列桶/拖库。
- 覆盖别人的对象或删除已有文件。
- 只凭版本号判断反序列化可利用。

## 主源
- `kb:file-upload-test`：§测试流程（开场）、STS/对象 key/签名/桶策略
- `kb:path-traversal-lfi-test`
- `kb:xxe-test`
- `kb:deserialization-test`
- `kb:jndi-injection-test`

## 次源
- `skill:offensive-file-upload`
- `skill:offensive-xxe`
- `skill:offensive-deserialization`

## 打开
```bash
python3 scripts/index.py show kb:file-upload-test --section "测试流程"
python3 scripts/index.py show kb:xxe-test
python3 scripts/index.py show skill:offensive-deserialization --section "Quick Workflow"
```
