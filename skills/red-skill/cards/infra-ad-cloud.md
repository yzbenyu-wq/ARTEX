# 决策卡：内网 / AD / K8s / 云

## 何时用
- 拿到内网入口、域账号、云凭证、容器权限。
- 目标涉及 AD/Kerberos、K8s、容器、AWS/Azure/GCP。
- 需要判断先打哪条链。

## 首打 5 步
1. AD：侦察域/用户/组 → 凭据收集 → 委派/ACL/ADCS → 横向。
2. K8s：枚举 → ServiceAccount token → RBAC → kubelet → 特权 Pod 逃逸。
3. 云：元数据 → IAM/角色 → 暴露的存储/函数/管理面。
4. 每条链只做到“证明当前权限边界被突破”，不继续扩散。
5. 记录身份、时间、请求、回包；方便复测与报告。

## 判定
- AD：拿到域控/目标主机权限，或有可复现的委派/ACL/ADCS 滥用链。
- K8s：跨命名空间/节点权限，或逃逸到宿主。
- 云：拿到目标账号/角色权限，或读到目标数据。

## 硬红线
- 只在授权内网/账号/集群；不做持久化、不横向扩散。
- 不导出无关数据；不修改生产配置。
- 影响预算优先于“打穿”。

## 常见坑
- 一上来就跑重型扫描，触发告警。
- 把 BloodHound 图当结论，没验证边。
- 在 K8s 里创建特权 Pod 后不清理。
- 云元数据能读就继续拿长期凭证。

## 主源
- `skill:offensive-active-directory`：§Reconnaissance、§ADCS Abuse、§Lateral Movement
- `skill:offensive-k8s-attacks`：§Phase 1、§Phase 2、§Phase 3、§Phase 5
- `skill:offensive-cloud`：§AWS、§Azure、§GCP

## 次源
- `skill:offensive-netexec`
- `skill:offensive-lateral-movement`
- `skill:offensive-windows-privesc`
- `skill:offensive-container-escape`

## 打开
```bash
python3 scripts/index.py show skill:offensive-active-directory --section "Reconnaissance"
python3 scripts/index.py show skill:offensive-k8s-attacks --section "Phase 1"
python3 scripts/index.py show skill:offensive-cloud --section "AWS"
```
