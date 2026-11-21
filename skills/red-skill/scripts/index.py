#!/usr/bin/env python3
"""Index and route the local red-team skill library.

Subcommands:
  update   scan sources, regenerate index.json / INDEX.md / CHANGELOG.md
  check    validate freshness, coverage, links, cards and golden routing
  find     keyword routing over the generated index
  show     print one bounded section of a source file
  stats    counts and size report

The source library is never modified.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterable, List, Optional, Tuple

try:
    import yaml
except ImportError:  # pragma: no cover
    yaml = None

SKILL_DIR = Path(__file__).resolve().parent.parent
DEFAULT_LIBRARY = SKILL_DIR.parent
INDEX_JSON = SKILL_DIR / "index.json"
INDEX_MD = SKILL_DIR / "INDEX.md"
CHANGELOG = SKILL_DIR / "CHANGELOG.md"
TAXONOMY = SKILL_DIR / "meta" / "taxonomy.yaml"
OVERRIDES = SKILL_DIR / "meta" / "overrides.yaml"
GOLDEN = SKILL_DIR / "tests" / "golden.json"
CARDS_DIR = SKILL_DIR / "cards"
SKILL_MD = SKILL_DIR / "SKILL.md"
LIBRARY_POINTER = SKILL_DIR / "meta" / "library.txt"
SRC_DIR_NAME = "00-src"
# ARTEX 交付形态：源库随技能一起分发，优先用技能内的 library/。
if (SKILL_DIR / "library" / SRC_DIR_NAME).exists():
    DEFAULT_LIBRARY = SKILL_DIR / "library"
SCHEMA_VERSION = 1
MAX_L0_LINES = 120
MAX_CARD_LINES = 60
TYPE_ORDER = {"src-entry": 0, "kb": 1, "rule": 2, "skill": 3}
TYPE_LABEL = {"src-entry": "SRC入口", "kb": "知识库", "rule": "规则", "skill": "技能"}


def die(msg: str, code: int = 2) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    sys.exit(code)


def load_yaml(path: Path) -> dict:
    if yaml is None:
        die("需要 PyYAML：python3 -m pip install pyyaml")
    if not path.exists():
        die(f"缺少配置文件：{path}")
    with path.open("r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
    if not isinstance(data, dict):
        die(f"配置格式错误（应为映射）：{path}")
    return data


def split_frontmatter(text: str) -> Tuple[str, str]:
    if not text.startswith("---"):
        return "", text
    end = text.find("\n---", 3)
    if end == -1:
        return "", text
    return text[3:end], text[end + 4 :]


def parse_frontmatter(text: str) -> Tuple[dict, str]:
    raw, body = split_frontmatter(text)
    meta: dict = {}
    if raw.strip():
        if yaml is not None:
            try:
                loaded = yaml.safe_load(raw)
                if isinstance(loaded, dict):
                    meta = loaded
            except Exception:
                meta = {}
        if not meta:
            for line in raw.splitlines():
                m = re.match(r"^(name|description):\s*(.*)$", line)
                if m:
                    meta[m.group(1)] = m.group(2).strip().strip("\"'")
    return meta, body


def clean_inline(value: str) -> str:
    value = re.sub(r"\[([^\]]+)\]\([^)]+\)", r"\1", value)
    value = re.sub(r"<[^>]+>", "", value)
    value = value.replace("`", "").replace("**", "").replace("__", "")
    value = re.sub(r"\s+", " ", value)
    return value.strip(" -|")


def truncate(value: str, limit: int) -> str:
    value = clean_inline(value)
    if len(value) <= limit:
        return value
    return value[: max(0, limit - 1)].rstrip() + "…"


def first_h1(body: str) -> str:
    m = re.search(r"^#\s+(.+?)\s*$", body, re.M)
    return clean_inline(m.group(1)) if m else ""


def first_paragraph(body: str) -> str:
    for block in re.split(r"\n\s*\n", body):
        lines = []
        for line in block.splitlines():
            stripped = line.strip()
            if not stripped:
                continue
            if stripped.startswith(("#", "|", "```", "---", ">")):
                continue
            lines.append(stripped)
        text = " ".join(lines)
        if len(clean_inline(text)) >= 12:
            return clean_inline(text)
    return ""


def table_descriptions(path: Path) -> Dict[str, str]:
    out: Dict[str, str] = {}
    if not path.exists():
        return out
    text = path.read_text(encoding="utf-8", errors="replace")
    for line in text.splitlines():
        if not line.strip().startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 2:
            continue
        names = re.findall(r"`([^`]+\.md)`", cells[0])
        desc = clean_inline(cells[1])
        if not desc:
            continue
        for name in names:
            out[name] = desc
    return out


def iter_sources(library: Path) -> Iterable[Tuple[str, Path]]:
    skill_files = []
    for path in sorted(library.rglob("SKILL.md")):
        rel = path.relative_to(library)
        if rel.parts and rel.parts[0] in {SRC_DIR_NAME, "red-skill"}:
            continue
        if any(part.startswith(".") for part in rel.parts):
            continue
        skill_files.append(path)
    for path in skill_files:
        yield "skill", path

    src_entry = library / SRC_DIR_NAME / "SKILL.md"
    if src_entry.exists():
        yield "src-entry", src_entry

    kb_dir = library / SRC_DIR_NAME / "知识库"
    if kb_dir.exists():
        for path in sorted(kb_dir.glob("*.md")):
            if path.name.lower() != "readme.md":
                yield "kb", path

    rules_dir = library / SRC_DIR_NAME / "rules"
    if rules_dir.exists():
        for path in sorted(rules_dir.glob("*.md")):
            yield "rule", path

    agents = library / SRC_DIR_NAME / "AGENTS.md"
    if agents.exists():
        yield "rule", agents


def estimate_tokens(text: str) -> int:
    cjk = len(re.findall(r"[\u4e00-\u9fff]", text))
    other = max(0, len(text) - cjk)
    return int(cjk / 1.5 + other / 4)


def dir_group_map(taxonomy: dict) -> Tuple[Dict[str, str], Dict[str, str], List[str]]:
    dir_to_group: Dict[str, str] = {}
    group_zh: Dict[str, str] = {}
    order: List[str] = []
    for group in taxonomy.get("groups", []):
        gid = str(group.get("id", "")).strip()
        if not gid:
            continue
        group_zh[gid] = str(group.get("zh", gid))
        order.append(gid)
        for d in group.get("dirs", []) or []:
            dir_to_group[str(d)] = gid
    return dir_to_group, group_zh, order


def unique_id(base: str, used: set) -> str:
    candidate = base
    n = 2
    while candidate in used:
        candidate = f"{base}-{n}"
        n += 1
    used.add(candidate)
    return candidate


def build_index(library: Path) -> dict:
    taxonomy = load_yaml(TAXONOMY)
    overrides = load_yaml(OVERRIDES)
    dir_to_group, group_zh, group_order = dir_group_map(taxonomy)

    kb_names = table_descriptions(library / SRC_DIR_NAME / "知识库" / "README.md")
    rule_names = table_descriptions(library / SRC_DIR_NAME / "AGENTS.md")

    trigger_map: Dict[str, List[str]] = {
        str(k): [str(x) for x in (v or [])] for k, v in (overrides.get("triggers") or {}).items()
    }
    summary_map: Dict[str, str] = {
        str(k): str(v) for k, v in (overrides.get("summaries") or {}).items()
    }
    verified_map: Dict[str, str] = {
        str(k): str(v) for k, v in (overrides.get("verified") or {}).items()
    }
    scenarios = overrides.get("scenarios") or []
    scenario_cards: Dict[str, str] = {}
    scenario_ids: Dict[str, List[str]] = {}
    for sc in scenarios:
        sid = str(sc.get("id", "")).strip()
        card = str(sc.get("card", "")).strip()
        ids = [str(x) for x in (sc.get("primary") or [])] + [str(x) for x in (sc.get("secondary") or [])]
        for entry_id in ids:
            scenario_ids.setdefault(entry_id, []).append(sid)
            if card and entry_id not in scenario_cards:
                scenario_cards[entry_id] = card
            trigger_map.setdefault(entry_id, [])
            trigger_map[entry_id].extend([str(x) for x in (sc.get("triggers") or [])])

    used: set = set()
    entries: List[dict] = []
    for kind, path in iter_sources(library):
        rel = path.relative_to(library).as_posix()
        raw = path.read_bytes()
        text = raw.decode("utf-8", errors="replace")
        meta, body = parse_frontmatter(text)
        name = str(meta.get("name") or path.parent.name).strip()
        title = first_h1(body) or name
        if kind == "skill":
            base_id = f"skill:{name}"
            group = dir_to_group.get(path.relative_to(library).parts[0], "other")
        elif kind == "src-entry":
            base_id = "src:entry"
            group = "src-entry"
        elif kind == "kb":
            base_id = f"kb:{path.stem}"
            group = "src-kb"
        else:
            base_id = f"rule:{path.stem}"
            group = "src-rules"
        entry_id = unique_id(base_id, used)

        description = str(meta.get("description") or "")
        if entry_id in summary_map:
            summary = summary_map[entry_id]
        elif kind == "kb":
            summary = kb_names.get(path.name) or first_paragraph(body) or description
        elif kind == "rule":
            summary = rule_names.get(path.name) or first_paragraph(body) or description
        else:
            summary = description or first_paragraph(body)

        triggers = {str(t) for t in trigger_map.get(entry_id, []) if str(t).strip()}
        triggers.add(group_zh.get(group, group))
        for word in re.findall(r"[A-Za-z][A-Za-z0-9_.+#-]{2,}", f"{name} {title}"):
            triggers.add(word.lower())
        triggers = sorted(triggers)[:40]

        lines = text.count("\n") + 1
        entry = {
            "id": entry_id,
            "type": kind,
            "name": name,
            "title": title,
            "group": group,
            "group_zh": group_zh.get(group, group),
            "path": rel,
            "summary": truncate(summary, 200),
            "triggers": triggers,
            "lines": lines,
            "bytes": len(raw),
            "sha256": hashlib.sha256(raw).hexdigest(),
            "est_tokens": estimate_tokens(text),
            "whole_ok": lines <= 300,
            "verified": verified_map.get(entry_id),
            "card": scenario_cards.get(entry_id),
            "scenarios": scenario_ids.get(entry_id, []),
        }
        entries.append(entry)

    def sort_key(e: dict) -> Tuple[int, int, str]:
        try:
            gidx = group_order.index(e["group"])
        except ValueError:
            gidx = len(group_order)
        return gidx, TYPE_ORDER.get(e["type"], 9), e["title"].lower()

    entries.sort(key=sort_key)
    counts: Dict[str, int] = {}
    for e in entries:
        counts[e["type"]] = counts.get(e["type"], 0) + 1
    groups = []
    for gid in group_order + ["other"]:
        n = sum(1 for e in entries if e["group"] == gid)
        if n:
            groups.append({"id": gid, "zh": group_zh.get(gid, gid), "count": n})

    # 用稳定标识而非绝对路径，保证宿主机与容器内生成的签名一致。
    bundled_library = (SKILL_DIR / "library").resolve()
    library_id = "library" if library.resolve() == bundled_library else str(library)
    config_sig = hashlib.sha256(
        library_id.encode("utf-8")
        + b"\0"
        + (TAXONOMY.read_bytes() if TAXONOMY.exists() else b"")
        + b"\0"
        + (OVERRIDES.read_bytes() if OVERRIDES.exists() else b"")
    ).hexdigest()

    return {
        "schema_version": SCHEMA_VERSION,
        "generated_at": "",
        "library": library_id,
        "config_sig": config_sig,
        "entry_count": len(entries),
        "counts": counts,
        "groups": groups,
        "scenarios": [
            {
                "id": str(sc.get("id", "")),
                "zh": str(sc.get("zh", "")),
                "primary": [str(x) for x in (sc.get("primary") or [])],
                "secondary": [str(x) for x in (sc.get("secondary") or [])],
                "triggers": [str(x) for x in (sc.get("triggers") or [])],
                "card": str(sc.get("card", "")),
                "note": str(sc.get("note", "")),
            }
            for sc in scenarios
        ],
        "entries": entries,
    }


def signature(entry: dict) -> tuple:
    return (
        entry.get("sha256"),
        entry.get("lines"),
        entry.get("group"),
        entry.get("summary"),
        tuple(entry.get("triggers") or []),
        entry.get("verified"),
        entry.get("card"),
        tuple(entry.get("scenarios") or []),
    )


def comparable(index: dict) -> Dict[str, tuple]:
    return {e["id"]: signature(e) for e in index.get("entries", [])}


def load_index() -> dict:
    if not INDEX_JSON.exists():
        die("index.json 不存在，先运行：python3 scripts/index.py update")
    with INDEX_JSON.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def esc_cell(value: str) -> str:
    return clean_inline(value).replace("|", "\\|")


def write_index_md(index: dict) -> None:
    counts = index["counts"]
    lines = [
        "# 红队 / SRC 技能总索引",
        "",
        "> 自动生成，勿手改。更新命令：`python3 scripts/index.py update`。",
        "",
        f"- 生成时间：{index.get('generated_at') or '未生成'}",
        f"- 条目总数：{index['entry_count']}（技能 {counts.get('skill', 0)} / SRC 入口 {counts.get('src-entry', 0)} / 知识库 {counts.get('kb', 0)} / 规则 {counts.get('rule', 0)}）",
        f"- 库根目录：`{index['library']}`",
        "",
        "## 常用场景路由",
        "",
        "| 场景 | 主源 | 次源 | 卡片 |",
        "|---|---|---|---|",
    ]
    for sc in index.get("scenarios", []):
        primary = " ".join(f"`{x}`" for x in sc["primary"])
        secondary = " ".join(f"`{x}`" for x in sc["secondary"])
        card = f"`{sc['card']}`" if sc["card"] else ""
        lines.append(f"| {esc_cell(sc['zh'])} | {primary} | {secondary} | {card} |")
    lines.append("")

    entries = index["entries"]
    for group in index["groups"]:
        gid = group["id"]
        lines.append(f"## {group['zh']}（{group['count']}）")
        lines.append("")
        lines.append("| ID | 摘要 | 行数 | 类型 | 路径 |")
        lines.append("|---|---|---:|---|---|")
        for entry in entries:
            if entry["group"] != gid:
                continue
            text = f"{entry['title']} — {entry['summary']}" if entry["summary"] else entry["title"]
            lines.append(
                f"| `{entry['id']}` | {esc_cell(truncate(text, 100))} | {entry['lines']} | "
                f"{TYPE_LABEL.get(entry['type'], entry['type'])} | `{entry['path']}` |"
            )
        lines.append("")

    INDEX_MD.write_text("\n".join(lines), encoding="utf-8")


def append_changelog(old: Optional[dict], new: dict, added: List[str], removed: List[str], changed: List[str], config_changed: bool = False) -> None:
    if not CHANGELOG.exists():
        CHANGELOG.write_text("# 索引变更记录\n\n", encoding="utf-8")
    stamp = datetime.now().strftime("%Y-%m-%d %H:%M")
    counts = new["counts"]
    block = [
        f"## {stamp}",
        "",
        f"- 条目：{new['entry_count']}（技能 {counts.get('skill', 0)} / SRC 入口 {counts.get('src-entry', 0)} / 知识库 {counts.get('kb', 0)} / 规则 {counts.get('rule', 0)}）",
        f"- 新增 {len(added)} / 删除 {len(removed)} / 变更 {len(changed)}",
    ]
    if old is None:
        block.append("- 初始索引")
    if config_changed:
        block.append("- 配置变更（taxonomy / overrides）")
    for label, items in (("新增", added), ("删除", removed), ("变更", changed)):
        if items:
            shown = ", ".join(items[:20])
            more = f" 等 {len(items)} 条" if len(items) > 20 else ""
            block.append(f"- {label}：{shown}{more}")
    with CHANGELOG.open("a", encoding="utf-8") as fh:
        fh.write("\n".join(block) + "\n\n")


def cmd_update(library: Path) -> int:
    new = build_index(library)
    old: Optional[dict] = None
    if INDEX_JSON.exists():
        try:
            with INDEX_JSON.open("r", encoding="utf-8") as fh:
                old = json.load(fh)
        except Exception:
            old = None

    old_map = comparable(old) if old else {}
    new_map = comparable(new)
    added = sorted(set(new_map) - set(old_map))
    removed = sorted(set(old_map) - set(new_map))
    changed = sorted(k for k in set(old_map) & set(new_map) if old_map[k] != new_map[k])
    config_changed = bool(old) and old.get("config_sig") != new["config_sig"]

    if old and not (added or removed or changed or config_changed) and INDEX_MD.exists() and CHANGELOG.exists():
        print(f"无变化：{new['entry_count']} 条，索引已是最新。")
        return 0

    new["generated_at"] = datetime.now().strftime("%Y-%m-%d %H:%M")
    INDEX_JSON.write_text(json.dumps(new, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    write_index_md(new)
    append_changelog(old, new, added, removed, changed, config_changed)
    print(
        f"已更新：{new['entry_count']} 条"
        f"（技能 {new['counts'].get('skill', 0)} / SRC {new['counts'].get('src-entry', 0)}"
        f" / 知识库 {new['counts'].get('kb', 0)} / 规则 {new['counts'].get('rule', 0)}）"
        f" | 新增 {len(added)} 删除 {len(removed)} 变更 {len(changed)}"
    )
    return 0


def read_l0_lines() -> int:
    if not SKILL_MD.exists():
        return 0
    return SKILL_MD.read_text(encoding="utf-8", errors="replace").count("\n") + 1


def headings(text: str, limit: int = 40) -> List[Tuple[int, int, str]]:
    out = []
    for m in re.finditer(r"^(#{1,4})\s+(.+?)\s*$", text, re.M):
        out.append((m.start(), len(m.group(1)), clean_inline(m.group(2))))
        if len(out) >= limit:
            break
    return out


def ascii_word_hit(term: str, query: str) -> bool:
    return re.search(rf"(?<![a-z0-9]){re.escape(term)}(?![a-z0-9])", query) is not None


def trigger_score(trigger: str, q: str) -> float:
    t = trigger.strip().lower()
    if not t:
        return 0.0
    if re.fullmatch(r"[a-z0-9_.+# -]+", t):
        if not ascii_word_hit(t, q):
            return 0.0
        return 2.5 if len(t) <= 3 else 6.0 + min(len(t), 16) * 0.8
    if t in q:
        return 6.0 + min(len(t), 16) * 0.8
    if len(t) >= 4 and q in t:
        return 3.0 + min(len(t), 16) * 0.3
    return 0.0


def cmd_find(library: Path, query: str, limit: int, as_json: bool) -> int:
    index = load_index()
    q = query.strip().lower()
    if not q:
        die("查询词为空")
    q_compact = re.sub(r"\s+", "", q)
    en_terms = set(re.findall(r"[a-z0-9][a-z0-9_.+#-]{1,}", q))
    zh_terms = set(re.findall(r"[\u4e00-\u9fff]{2,}", q))
    terms = en_terms | zh_terms
    for phrase in zh_terms:
        for i in range(len(phrase) - 1):
            terms.add(phrase[i : i + 2])

    boosts: Dict[str, float] = {}
    for sc in index.get("scenarios") or []:
        if not any(trigger_score(t, q) > 0 for t in sc.get("triggers") or []):
            continue
        for i, entry_id in enumerate(sc.get("primary") or []):
            boosts[entry_id] = max(boosts.get(entry_id, 0.0), 6.0 - i * 0.3)
        for entry_id in sc.get("secondary") or []:
            boosts[entry_id] = max(boosts.get(entry_id, 0.0), 3.0)

    results = []
    for entry in index["entries"]:
        score = boosts.get(entry["id"], 0.0)
        for trigger in entry.get("triggers") or []:
            score += trigger_score(trigger, q)
        name_low = entry["name"].lower().replace("-", " ")
        if len(q_compact) >= 3 and (q_compact in name_low.replace(" ", "") or name_low in q):
            score += 10.0
        title_low = entry["title"].lower()
        summary_low = entry["summary"].lower()
        title_hits = 0
        summary_hits = 0
        for term in terms:
            if len(term) < 2:
                continue
            if term in title_low and title_hits < 3:
                score += 3.0
                title_hits += 1
            if term in summary_low and summary_hits < 5:
                score += 1.0
                summary_hits += 1
        if entry.get("group_zh", "").lower() in q:
            score += 2.0
        score += {"kb": 1.5, "src-entry": 1.5, "skill": 1.0, "rule": 0.5}.get(entry["type"], 0.0)
        if entry.get("verified"):
            score += 0.5
        if score > 0:
            results.append((score, entry))

    results.sort(key=lambda x: (-x[0], TYPE_ORDER.get(x[1]["type"], 9), x[1]["lines"]))
    top = results[:limit]
    if as_json:
        print(json.dumps(
            [
                {
                    "score": round(score, 1),
                    "id": e["id"],
                    "type": e["type"],
                    "lines": e["lines"],
                    "path": e["path"],
                    "summary": e["summary"],
                    "card": e.get("card"),
                }
                for score, e in top
            ],
            ensure_ascii=False,
            indent=2,
        ))
        return 0
    if not top:
        print("没有命中。换关键词，或运行 `index.py stats` 看覆盖范围。")
        return 1
    for score, e in top:
        card = f"  card={e['card']}" if e.get("card") else ""
        print(f"{score:6.1f}  {e['id']:<44} {e['type']:<9} {e['lines']:>6}  {e['path']}{card}")
        print(f"        {truncate(e['summary'], 120)}")
    return 0


def resolve_entry(index: dict, arg: str) -> Optional[dict]:
    for entry in index["entries"]:
        if entry["id"] == arg or entry["name"] == arg or entry["path"] == arg:
            return entry
    matches = [e for e in index["entries"] if e["id"].endswith(arg) or e["path"].endswith(arg)]
    return matches[0] if len(matches) == 1 else None


def normalize_heading(value: str) -> str:
    value = value.lower()
    value = re.sub(r"[\s#`*_:：、，,。.\-—()（）\[\]【】]+", "", value)
    return value


def cmd_show(library: Path, arg: str, section: Optional[str], max_lines: int, force: bool) -> int:
    index = load_index()
    entry = resolve_entry(index, arg)
    if entry:
        path = library / entry["path"]
        entry_id = entry["id"]
        rel = entry["path"]
    else:
        direct = Path(arg)
        if not direct.is_absolute():
            candidates = [SKILL_DIR / arg, library / arg]
        else:
            candidates = [direct]
        path = next((p for p in candidates if p.exists() and p.is_file()), None)  # type: ignore[assignment]
        if not path:
            die(f"找不到条目或文件：{arg}")
        entry_id = "file"
        rel = str(path.relative_to(library)) if str(path).startswith(str(library)) else str(path)

    text = path.read_text(encoding="utf-8", errors="replace")
    total = text.count("\n") + 1
    if section:
        items = headings(text, limit=200)
        want = normalize_heading(section)
        target = None
        for pos, level, title in items:
            if want and (want in normalize_heading(title) or normalize_heading(title) in want):
                target = (pos, level, title)
                break
        if not target:
            print(f"未找到章节：{section}")
            print("可用章节：")
            for _, _, title in items[:25]:
                print(f"  - {title}")
            return 1
        start, level, title = target
        end = len(text)
        for pos, next_level, _ in items:
            if pos > start and next_level <= level:
                end = pos
                break
        chunk = text[start:end].rstrip().splitlines()
        start_line = text[:start].count("\n") + 1
        if len(chunk) > max_lines and not force:
            chunk = chunk[:max_lines] + [f"... [已截断，原节 {len(chunk)} 行；需要更多用 --max-lines N]"]
        print(f"# id: {entry_id}")
        print(f"# source: {rel}")
        print(f"# lines: {start_line}-{start_line + len(chunk) - 1} / {total}  section: {title}")
        print()
        print("\n".join(chunk))
        return 0

    lines = text.splitlines()
    if len(lines) > 300 and not force:
        print(f"拒绝整篇输出：{entry_id}（{total} 行）。请用 --section \"章节名\" 只读一节。")
        print("可用章节：")
        for _, _, title in headings(text)[:25]:
            print(f"  - {title}")
        return 1
    print(f"# id: {entry_id}")
    print(f"# source: {rel}")
    print(f"# lines: 1-{min(len(lines), max_lines)} / {total}")
    print()
    if len(lines) <= max_lines:
        print("\n".join(lines))
        return 0
    print("\n".join(lines[:max_lines]))
    print(f"... [已截断，共 {total} 行；用 --section 或 --max-lines 继续]")
    return 0


def cmd_stats(library: Path) -> int:
    index = load_index()
    print(f"条目：{index['entry_count']}  {index['counts']}")
    print(f"库根：{index['library']}")
    print(f"L0：{read_l0_lines()} 行（上限 {MAX_L0_LINES}）")
    total_lines = sum(e["lines"] for e in index["entries"])
    total_tokens = sum(e["est_tokens"] for e in index["entries"])
    print(f"源文件总行数：{total_lines}，估算 token：{total_tokens}")
    print("最大 10 个源：")
    for e in sorted(index["entries"], key=lambda x: x["lines"], reverse=True)[:10]:
        print(f"  {e['lines']:>6}  {e['id']:<44} {e['path']}")
    cards = sorted(CARDS_DIR.glob("*.md"))
    print(f"卡片：{len(cards)} 张，最大 {max((c.read_text(encoding='utf-8').count(chr(10)) + 1 for c in cards), default=0)} 行")
    return 0


def check_kb_links(library: Path) -> List[str]:
    errors = []
    readme = library / SRC_DIR_NAME / "知识库" / "README.md"
    if not readme.exists():
        return errors
    text = readme.read_text(encoding="utf-8", errors="replace")
    names = set(re.findall(r"`([^`]+\.md)`", text))
    kb_dir = library / SRC_DIR_NAME / "知识库"
    for name in names:
        if name.lower() in {"readme.md", "skill.md"} or "*" in name or name.startswith("~") or "/" in name:
            continue
        if not (kb_dir / name).exists():
            errors.append(f"KB README 断链：{name}")
    return errors


def cmd_check(library: Path, run_golden: bool) -> int:
    failures: List[str] = []
    if not INDEX_JSON.exists():
        die("index.json 不存在，先运行 update")

    old = load_index()
    fresh = build_index(library)
    old_map = comparable(old)
    new_map = comparable(fresh)
    stale = sorted(
        k
        for k in set(old_map) | set(new_map)
        if old_map.get(k) != new_map.get(k)
    )
    if stale:
        failures.append(f"索引过期：{len(stale)} 条源文件已变化，先运行 update（例：{', '.join(stale[:5])}）")

    if old.get("entry_count") != fresh["entry_count"]:
        failures.append(f"条目数不一致：索引 {old.get('entry_count')} / 实际 {fresh['entry_count']}")
    if old.get("config_sig") != fresh["config_sig"]:
        failures.append("索引过期：taxonomy / overrides 已变化，先运行 update")

    ids = set()
    for entry in fresh["entries"]:
        if entry["id"] in ids:
            failures.append(f"重复 ID：{entry['id']}")
        ids.add(entry["id"])
        if not (library / entry["path"]).exists():
            failures.append(f"源文件缺失：{entry['path']}")
        if not entry["summary"]:
            failures.append(f"摘要为空：{entry['id']}")
        if entry["lines"] < 1:
            failures.append(f"行数异常：{entry['id']}")

    overrides = load_yaml(OVERRIDES)
    for sc in overrides.get("scenarios", []) or []:
        sid = sc.get("id", "?")
        for key in ("primary", "secondary"):
            for entry_id in sc.get(key, []) or []:
                if entry_id not in ids:
                    failures.append(f"场景 {sid} 的 {key} 引用了不存在的 ID：{entry_id}")
        card = str(sc.get("card", "")).strip()
        if card and not (SKILL_DIR / card).exists():
            failures.append(f"场景 {sid} 的卡片不存在：{card}")
    for key in ("triggers", "summaries", "verified"):
        for entry_id in (overrides.get(key) or {}):
            if entry_id not in ids:
                failures.append(f"overrides.{key} 引用了不存在的 ID：{entry_id}")

    l0_lines = read_l0_lines()
    if l0_lines > MAX_L0_LINES:
        failures.append(f"SKILL.md 超过 {MAX_L0_LINES} 行：{l0_lines}")

    referenced_cards = {
        str(sc.get("card", "")).strip()
        for sc in overrides.get("scenarios", []) or []
        if str(sc.get("card", "")).strip()
    }
    cards = sorted(CARDS_DIR.glob("*.md"))
    for card in cards:
        n = card.read_text(encoding="utf-8", errors="replace").count("\n") + 1
        if n > MAX_CARD_LINES:
            failures.append(f"卡片超过 {MAX_CARD_LINES} 行：{card.name}（{n}）")
        rel = f"cards/{card.name}"
        if rel not in referenced_cards:
            failures.append(f"卡片未被任何场景引用：{rel}")
    for rel in referenced_cards:
        if not (SKILL_DIR / rel).exists():
            failures.append(f"场景引用的卡片缺失：{rel}")

    if INDEX_MD.exists():
        md = INDEX_MD.read_text(encoding="utf-8", errors="replace")
        missing = [entry["id"] for entry in fresh["entries"] if f"`{entry['id']}`" not in md]
        if missing:
            failures.append(f"INDEX.md 缺少 {len(missing)} 个条目（例：{', '.join(missing[:5])}）")
    else:
        failures.append("INDEX.md 不存在")

    failures.extend(check_kb_links(library))

    if run_golden and GOLDEN.exists():
        tests = json.loads(GOLDEN.read_text(encoding="utf-8"))
        for case in tests:
            query = case["query"]
            expected = [str(x) for x in case["expect"]]
            limit = int(case.get("top", 3))
            output = json.loads(capture_find(library, query, limit))
            got = [x["id"] for x in output]
            ok = bool(expected) and expected[0] in got[:limit]
            mark = "PASS" if ok else "FAIL"
            print(f"  [{mark}] {query} -> {', '.join(got[:limit])}")
            if not ok:
                failures.append(f"黄金用例失败：{query}（首选 {expected[0] if expected else '-'}，实际 {got[:limit]}）")
    elif run_golden:
        failures.append("tests/golden.json 不存在")

    if failures:
        print(f"CHECK FAILED（{len(failures)}）")
        for item in failures:
            print(f"  - {item}")
        return 1
    print(
        f"CHECK OK：{fresh['entry_count']} 条，L0 {l0_lines} 行，"
        f"卡片 {len(cards)} 张，覆盖 {len(fresh['groups'])} 组"
    )
    return 0


def capture_find(library: Path, query: str, limit: int) -> str:
    import io
    from contextlib import redirect_stdout

    buf = io.StringIO()
    with redirect_stdout(buf):
        cmd_find(library, query, limit, True)
    return buf.getvalue()


def resolve_library(value: Optional[str]) -> Path:
    if value:
        path = Path(value).expanduser().resolve()
    elif os.environ.get("RED_LIBRARY"):
        path = Path(os.environ["RED_LIBRARY"]).expanduser().resolve()
    elif LIBRARY_POINTER.exists():
        raw = LIBRARY_POINTER.read_text(encoding="utf-8").strip()
        if not raw:
            die(f"库路径指针为空：{LIBRARY_POINTER}")
        path = Path(raw).expanduser().resolve()
        if not path.exists():
            bundled = (SKILL_DIR / "library").resolve()
            if bundled.exists():
                path = bundled
    else:
        path = DEFAULT_LIBRARY
    if not path.exists():
        die(f"库根目录不存在：{path}（用 --library 或 RED_LIBRARY 指定）")
    if not (path / SRC_DIR_NAME / "SKILL.md").exists():
        die(f"不是有效源库：{path}（缺少 {SRC_DIR_NAME}/SKILL.md）")
    return path


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description="红队 / SRC 技能库索引工具")
    parser.add_argument("--library", help="库根目录（默认技能内 library/，可用 RED_LIBRARY 覆盖）")
    sub = parser.add_subparsers(dest="command", required=True)

    p_update = sub.add_parser("update", help="扫描并重建索引")
    p_update.add_argument("--library", help=argparse.SUPPRESS)

    p_check = sub.add_parser("check", help="校验索引、覆盖率与路由")
    p_check.add_argument("--no-golden", action="store_true", help="跳过黄金用例")
    p_check.add_argument("--library", help=argparse.SUPPRESS)

    p_find = sub.add_parser("find", help="按关键词路由")
    p_find.add_argument("query")
    p_find.add_argument("-n", "--limit", type=int, default=5)
    p_find.add_argument("--json", action="store_true")
    p_find.add_argument("--library", help=argparse.SUPPRESS)

    p_show = sub.add_parser("show", help="输出条目或单节内容")
    p_show.add_argument("id")
    p_show.add_argument("--section")
    p_show.add_argument("--max-lines", type=int, default=80)
    p_show.add_argument("--force", action="store_true", help="允许超长整篇输出（慎用）")
    p_show.add_argument("--library", help=argparse.SUPPRESS)

    p_stats = sub.add_parser("stats", help="统计覆盖与体积")
    p_stats.add_argument("--library", help=argparse.SUPPRESS)

    args = parser.parse_args(argv)
    library = resolve_library(args.library)
    if args.command == "update":
        return cmd_update(library)
    if args.command == "check":
        return cmd_check(library, not args.no_golden)
    if args.command == "find":
        return cmd_find(library, args.query, args.limit, args.json)
    if args.command == "show":
        return cmd_show(library, args.id, args.section, args.max_lines, args.force)
    if args.command == "stats":
        return cmd_stats(library)
    return 2


if __name__ == "__main__":
    sys.exit(main())
