# 决策卡：报告 / 定级 / 复现

## 何时用
- 漏洞已复现，准备交付 SRC/客户报告。
- 需要定级、写复现步骤、整理证据。
- 不确定该不该提交、提交到哪。

## 首打 5 步
1. 先确认可复现：同一入口、同一参数、同一身份，至少两次。
2. 按固定版式写 8 块：标题、范围、前置条件、复现、影响、证据、修复、参考。
3. 定级只用 `rule:vuln-report-format` 的表，不自己发明标准。
4. 证据脱敏：密钥/Token/Cookie/个人信息只留片段；每步带时间戳。
5. 修复后复测，记录复测结果；不能复测就标注。

## 判定
- 只认 `rule:vuln-report-format`；报告版式与定级不在知识库里另找。
- 证据不足、影响说不清 → 先补证据，不堆低危。
- 不虚高 CVSS；不把“可能”写成“已确认”。

## 常见坑
- 提交一堆低危充数。
- 复现步骤缺前置条件/账号/环境。
- 截图带真实密钥、手机号、身份证。
- 影响描述只写“可导致泄露”，不说泄露什么、谁能利用。

## 主源
- `rule:vuln-report-format`：§正文怎么写、§落不落/落哪
- `skill:offensive-reporting`

## 次源
- `kb:info-leak-test`（信息泄露类证据与影响）

## 打开
```bash
python3 scripts/index.py show rule:vuln-report-format --section "正文怎么写"
python3 scripts/index.py show rule:vuln-report-format --section "落不落"
python3 scripts/index.py show skill:offensive-reporting --section "Quick Workflow"
```
