"use client";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useUITranslation } from "@/i18n/use-ui-translation";
import { apiFetch, openSSE } from "@/lib/api";
import { appendLive, mergeOlder } from "@/lib/log-merge";
import { MOCK } from "@/lib/mock/enabled";
import type { LogLine } from "@/lib/types";
import { cn } from "@/lib/utils";

// Mock demo：无后端 SSE，塞几行示例日志。
const MOCK_LOGS: LogLine[] = [
  {
    seq: 1,
    ts: "2026-07-26T03:55:00Z",
    level: "info",
    tag: "engine",
    text: "ARTEX v0.1.0 backend listening on :8787 (workers=3)",
  },
  {
    seq: 2,
    ts: "2026-07-26T03:55:01Z",
    level: "info",
    tag: "config",
    text: "LLM configured from DB: anthropic / claude-opus-4-8",
  },
  {
    seq: 3,
    ts: "2026-07-26T03:56:10Z",
    level: "info",
    tag: "planner",
    text: "task t-acme-web: 第 3 轮规划，生成意图 i-4",
  },
  {
    seq: 4,
    ts: "2026-07-26T03:57:00Z",
    level: "warn",
    tag: "guard",
    text: "block bash: 目标越界 out.evil.example 不在 scope 内",
  },
  {
    seq: 5,
    ts: "2026-07-26T03:57:30Z",
    level: "info",
    tag: "work#1",
    text: "report_finding: Default Credentials (high) 已落库",
  },
  {
    seq: 6,
    ts: "2026-07-26T03:58:20Z",
    level: "error",
    tag: "work#3",
    text: "intercept: mysqldump 命中破坏性规则，等待人工审批",
  },
];

const levelTone: Record<LogLine["level"], string> = {
  info: "text-foreground/80",
  warn: "text-amber-600 dark:text-amber-400",
  error: "text-red-600 dark:text-red-400",
};
const levelDot: Record<LogLine["level"], string> = {
  info: "bg-muted-foreground/40",
  warn: "bg-amber-500",
  error: "bg-red-500",
};

// 暂停缓冲上限，与实时窗口一致：长时间暂停时只保留最近 3000 行，不无限吃内存。
const PAUSE_BUFFER_MAX = 3000;

function fmtTime(ts: string) {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

export default function LogsPage() {
  const { ui } = useUITranslation();

  const [lines, setLines] = React.useState<LogLine[]>([]);
  const [q, setQ] = React.useState("");
  const [level, setLevel] = React.useState<"all" | LogLine["level"]>("all");
  const [paused, setPaused] = React.useState(false);
  const [loadingHistory, setLoadingHistory] = React.useState(false);
  // 初始按"服务端可能还有更早日志"处理；MOCK 演示没有后端，不显示按钮。
  const [hasMore, setHasMore] = React.useState(!MOCK);
  const bottom = React.useRef<HTMLDivElement>(null);
  const stick = React.useRef(true);
  const pausedRef = React.useRef(false);
  pausedRef.current = paused;
  // 服务端进程标识：重启后 seq 从 1 重数，发现 epoch 变化就整屏重置。
  const bootRef = React.useRef("");
  // 历史分页锚点：独立于渲染行，因为实时行永远没有 db_id。
  const oldestDbIdRef = React.useRef(0);
  // 暂停期间到达的行先攒着，恢复时一次性并入，避免暂停即永久丢日志。
  const pausedBuf = React.useRef<LogLine[]>([]);

  // Live tail via SSE. on connect, replays whatever is in the ring (includes
  // DB-restored history from the last 100 rows written before restart).
  React.useEffect(() => {
    if (MOCK) {
      setLines(MOCK_LOGS);
      return;
    }
    const es = openSSE("/api/logs/stream?since=0");
    es.onmessage = (e) => {
      try {
        const l = JSON.parse(e.data) as LogLine;
        if (l.epoch && l.epoch !== bootRef.current) {
          bootRef.current = l.epoch;
          oldestDbIdRef.current = 0;
          pausedBuf.current = [];
          setHasMore(true);
          setLines([l]);
        }
        // 重启恢复的历史行带 db_id，用它们把分页锚点钉在所有实时行之前；
        // 暂停时也照样更新，否则恢复后锚点会落后并造成重复。
        if (l.db_id && (oldestDbIdRef.current === 0 || l.db_id < oldestDbIdRef.current)) {
          oldestDbIdRef.current = l.db_id;
        }
        if (pausedRef.current) {
          const buf = pausedBuf.current;
          buf.push(l);
          if (buf.length > PAUSE_BUFFER_MAX) buf.splice(0, buf.length - PAUSE_BUFFER_MAX);
          return;
        }
        setLines((prev) => appendLive(prev, [l]));
      } catch {
        /* ignore malformed frame */
      }
    };
    return () => es.close();
  }, []);

  async function loadOlderHistory() {
    if (loadingHistory) return;
    setLoadingHistory(true);
    try {
      // 锚点来自历史接口自己返回的最小 db_id，而不是渲染行——实时行没有 db_id。
      const before = oldestDbIdRef.current;
      const params = before > 0 ? `?before=${before}&limit=200` : `?limit=200`;
      const res = await apiFetch(`/api/logs/history${params}`);
      if (!res.ok) return;
      const data = (await res.json()) as { items: LogLine[]; has_more: boolean };
      const items = data.items ?? [];
      if (items.length) {
        let minId = 0;
        for (const l of items) {
          if (l.db_id && (minId === 0 || l.db_id < minId)) minId = l.db_id;
        }
        // 即使整批都被去重也要推进锚点，否则同一批 200 行会被反复请求。
        if (minId > 0) oldestDbIdRef.current = minId;
        setLines((prev) => mergeOlder(prev, items, before === 0));
      }
      setHasMore(items.length > 0 && (data.has_more ?? false));
    } finally {
      setLoadingHistory(false);
    }
  }

  // 恢复时先把暂停期间攒下的行并入，再解除暂停，避免这一瞬间的新帧插队。
  function togglePause() {
    if (paused) {
      const pending = pausedBuf.current;
      pausedBuf.current = [];
      if (pending.length > 0) setLines((prev) => appendLive(prev, pending));
    }
    setPaused((p) => !p);
  }

  const filtered = React.useMemo(() => {
    const needle = q.trim().toLowerCase();
    return lines.filter(
      (l) =>
        (level === "all" || l.level === level) &&
        (!needle || l.text.toLowerCase().includes(needle) || l.tag.toLowerCase().includes(needle)),
    );
  }, [lines, q, level]);

  // biome-ignore lint/correctness/useExhaustiveDependencies: filtered is the scroll trigger; the effect only reads refs.
  React.useEffect(() => {
    if (stick.current && !paused) bottom.current?.scrollIntoView();
  }, [filtered, paused]);

  const counts = React.useMemo(() => {
    let warn = 0,
      error = 0;
    for (const l of lines) {
      if (l.level === "warn") warn++;
      else if (l.level === "error") error++;
    }
    return { warn, error, total: lines.length };
  }, [lines]);

  return (
    <div className="flex flex-1 flex-col gap-3">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">{ui("screen.366ada1d2fcfc4")}</h1>
        <p className="text-muted-foreground text-sm">{ui("screen.2420e315e38662")}</p>
      </div>
      <div className="flex flex-1 flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Input
            placeholder={ui("screen.b154738eea7450")}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            className="h-8 max-w-xs"
          />
          <div className="flex gap-1">
            {(["all", "info", "warn", "error"] as const).map((lv) => (
              <Button
                key={lv}
                size="sm"
                variant={level === lv ? "default" : "outline"}
                className="h-8"
                onClick={() => setLevel(lv)}
              >
                {lv === "all" ? ui("screen.5c55a67935af8f") : lv}
              </Button>
            ))}
          </div>
          <Button size="sm" variant={paused ? "default" : "outline"} className="h-8" onClick={togglePause}>
            {paused ? ui("screen.eb0c326b60ae89") : ui("screen.8d12fc0d4eb260")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="h-8"
            onClick={() => {
              oldestDbIdRef.current = 0;
              pausedBuf.current = [];
              setHasMore(true);
              setLines([]);
            }}
          >
            {ui("screen.1ef3de06b32e52")}
          </Button>
          <span className="ml-auto text-xs text-muted-foreground">
            {counts.total} {ui("screen.7049a63d1e7231")}{" "}
            <span className="text-amber-600 dark:text-amber-400">
              {counts.warn} {ui("screen.a8b7a4480407ac")}
            </span>{" "}
            ·{" "}
            <span className="text-red-600 dark:text-red-400">
              {counts.error} {ui("screen.0bc1fb72ae1be5")}
            </span>
          </span>
        </div>

        <div
          onScroll={(e) => {
            const el = e.currentTarget;
            stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
          }}
          className="h-[calc(100vh-14rem)] overflow-auto rounded-lg border bg-card p-2 font-mono text-xs leading-relaxed"
        >
          {hasMore && (
            <div className="flex justify-center py-2">
              <Button
                size="sm"
                variant="outline"
                className="h-7 text-xs"
                disabled={loadingHistory}
                onClick={loadOlderHistory}
              >
                {loadingHistory ? ui("screen.4927a53bcc886a") : ui("screen.ee2719de652ef1")}
              </Button>
            </div>
          )}
          {filtered.length === 0 ? (
            <p className="py-10 text-center text-muted-foreground">{ui("screen.abfa829f432e87")}</p>
          ) : (
            filtered.map((l) => {
              const body =
                l.tag && l.text.startsWith("[" + l.tag + "]") ? l.text.slice(l.tag.length + 2).trimStart() : l.text;
              return (
                <div key={l.seq} className="flex items-start gap-2 px-1 py-0.5 hover:bg-muted/40">
                  <span className={cn("mt-1.5 size-1.5 shrink-0 rounded-full", levelDot[l.level])} />
                  <span className="shrink-0 tabular-nums text-muted-foreground">{fmtTime(l.ts)}</span>
                  {l.tag && (
                    <span className="shrink-0 rounded bg-muted px-1 text-[10px] text-foreground/70">{l.tag}</span>
                  )}
                  <span className={cn("min-w-0 whitespace-pre-wrap break-words", levelTone[l.level])}>{body}</span>
                </div>
              );
            })
          )}
          <div ref={bottom} />
        </div>
      </div>
    </div>
  );
}
