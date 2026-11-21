# red-skill · 维护手册

单入口红队 / SRC 技能：生成式索引 + 12 张中文决策卡 + 按节抽取原文。
源库已整体收进 `library/`，按 12 个领域目录 + `00-src` 分类，内部文件保持原样。

## 环境

- Python 3.9+，需 PyYAML（`python3 -c "import yaml"` 自检）。
- 无网络依赖，不自动拉上游。

## 目录

```
red-skill/
├── SKILL.md            # L0 路由（≤120 行，唯一常驻上下文）
├── INDEX.md            # 人类可读总表（勿整体进上下文）
├── index.json          # 机器索引（含 SHA256 / 行数 / 摘要 / 路由）
├── CHANGELOG.md        # 每次 update 的增删改记录
├── cards/              # 12 张决策卡（每张 ≤60 行）
├── meta/taxonomy.yaml  # 16 个领域分组 → 源目录
├── meta/overrides.yaml # 场景路由 / 摘要 / 卡片指针 / verified 日期
├── tests/golden.json   # 12 条黄金路由用例（hit@3）
├── scripts/index.py    # 唯一工具入口
├── scripts/daily.sh    # update + check，写日志
├── install.sh          # 软链或复制安装
├── logs/               # daily.sh 输出
└── library/            # 源库（自包含）
    ├── 00-src/         # SRC 入口 / 知识库48 / rules11 / AGENTS.md
    ├── 01-web-api/     # web api api-security auth php2shell
    ├── 02-recon/       # recon bbot-recon
    ├── 03-cloud-container/
    ├── 04-internal/    # AD 内网 提权 后渗透 基础设施
    ├── 05-mobile-iot-wireless/
    ├── 06-binary/      # 利用 逆向 Fuzz 代码审计 补丁 diff
    ├── 07-supply-cicd/
    ├── 08-social/
    ├── 09-forensics/
    ├── 10-crypto/
    ├── 11-ai/
    └── 12-utility/
```

分类只影响目录位置，不改源文件内容；检索仍按 `index.json` 的 ID 与场景路由。

## 常用命令

本技能自包含，`library/` 就在技能目录内；ARTEX 会在运行环境注入 `SKILL_DIR`，
独立使用时把它设为本目录的绝对路径。

```bash
SKILL_DIR=/path/to/skills/red-skill

python3 "$SKILL_DIR/scripts/index.py" find "有登录态怎么测越权"
python3 "$SKILL_DIR/scripts/index.py" find "K8s 容器逃逸" --json
python3 "$SKILL_DIR/scripts/index.py" show kb:idor-test --section "测试流程"
python3 "$SKILL_DIR/scripts/index.py" stats
python3 "$SKILL_DIR/scripts/index.py" update    # 增量收录；无变化不写文件
python3 "$SKILL_DIR/scripts/index.py" check     # 全量门禁，失败退出 1
```

`show` 规则：≤80 行整篇输出；81–300 行只给前 80 行 + 目录；>300 行必须
`--section`，否则拒绝。节输出默认截断 80 行，可用 `--max-lines` 调整。

## 日常更新

1. 新增英文技能：放进源库任意分类夹（含 `SKILL.md`），跑 `update`。
2. 新增中文 KB / 规则：放进 `skills/知识库/` 或 `skills/rules/`，跑 `update`。
3. 新增或调整场景、主源、摘要：只改 `meta/overrides.yaml`，再 `update`。
4. 新增卡片：写到 `cards/`，并在 `overrides.yaml` 某场景 `card:` 引用，否则 `check` 失败。
5. 每次改完必须 `check` 通过；黄金用例失败先修路由，不提交索引。

定时（可选，不自动安装）：

```cron
17 8 * * * /path/to/skills/red-skill/scripts/daily.sh
```

## 新增一条路由

```yaml
scenarios:
  - id: my-scene
    zh: 场景中文名
    triggers: [关键词1, 关键词2]
    primary: [kb:xxx, skill:yyy]   # 必须存在于索引
    secondary: [rule:zzz]
    card: my-scene                 # 可选，指向 cards/my-scene.md
    note: 一句话边界说明
```

`verified: "YYYY-MM-DD"` 只标在“我实际核对过章节/路由”的条目上，不要批量刷日期。

## 优先级与安全

用户授权/范围 > `skills/rules` > 中文 KB > 英文深挖 > 卡片。
CORS 不挖；报告只认 `vuln-report-format`；仅授权目标，禁止批量扫描、DoS、
破坏性利用、真实资损、登出会话、记录密钥。

## 安装

```bash
./install.sh                 # 软链到 ~/.qoder-cn/skills/red-skill
./install.sh --to /path/dir  # 指定目标
./install.sh --copy          # 复制而非软链（如装进 ARTEX）
```

已存在真实目录时脚本拒绝覆盖；目标已是本目录软链则幂等退出 0。
卸载：删除该软链即可，源库不受影响。

## 排错

- `ModuleNotFoundError: yaml` → `pip3 install pyyaml`。
- `check` 报场景引用不存在 → 看 `find` 输出确认 ID，或先 `update`。
- 索引过期 → 直接 `update`；不要手改 `index.json` / `INDEX.md`。
- 源库默认就是技能内的 `library/`；确需引用外部源库时设 `RED_LIBRARY`，不要新增指针文件。
- 本目录是唯一事实源；不要再保留第二份副本，改动只提交仓库内的这一份。
