import type { LogLine } from "./types";

// 实时行不带 db_id（写入成功后才回填到 ring，广播早已发出），所以历史去重必须
// 同时认内容键，否则同一行会以"实时版 + 历史版"出现两次。
export function lineKey(l: LogLine): string {
  return l.db_id ? `d${l.db_id}` : `t${l.ts}|${l.level}|${l.tag}|${l.text}`;
}

// 不含时间戳的键：实时行的 ts 是采集时间，历史行是落库时间，两者可能差 1 秒，
// 只有内容能对上。仅用于"没有历史锚点"时的首屏去重。
export function looseKey(l: LogLine): string {
  return `${l.level}|${l.tag}|${l.text}`;
}

// mergeOlder 把一批更早的历史行前插到 prev，去掉与已有行重复的条目，再给新行
// 分配低于当前最小 seq 的合成 seq（它们只用于 React key 和实时去重）。
// loose=true 时额外按内容去重，用于 before 未知（锚点仍为 0）的首屏加载。
export function mergeOlder(prev: LogLine[], older: LogLine[], loose: boolean): LogLine[] {
  if (older.length === 0) return prev;
  const exact = new Set(prev.map(lineKey));
  const rough = loose ? new Set(prev.map(looseKey)) : null;
  const picked: LogLine[] = [];
  for (const l of older) {
    const key = lineKey(l);
    if (exact.has(key)) continue;
    if (rough) {
      const content = looseKey(l);
      if (rough.has(content)) continue;
      rough.add(content);
    }
    exact.add(key);
    picked.push(l);
  }
  if (picked.length === 0) return prev;
  const minSeq = prev.reduce((m, l) => Math.min(m, l.seq), 0);
  // 合成 seq 全部落在 0 以下的连续区间，也不会与下一批重叠。
  const restamped = picked.map((l, i) => ({ ...l, seq: minSeq - picked.length + i }));
  return [...restamped, ...prev].slice(-5000);
}
