import { appendLive, mergeOlder } from "./log-merge.ts";
import assert from "node:assert/strict";
import test from "node:test";

const line = (seq, extra = {}) => ({
  seq,
  ts: "2026-11-22T10:00:00Z",
  level: "info",
  tag: "pg",
  text: `line-${seq}`,
  ...extra,
});

test("history batch is prepended with unique synthetic seqs", () => {
  const live = [line(9), line(10)];
  const older = [line(0, { db_id: 101, text: "a" }), line(0, { db_id: 102, text: "b" })];

  const merged = mergeOlder(live, older, false);

  assert.deepEqual(
    merged.map((l) => l.text),
    ["a", "b", "line-9", "line-10"],
  );
  const seqs = merged.map((l) => l.seq);
  assert.equal(new Set(seqs).size, seqs.length, "seqs must be unique (React key)");
  assert.ok(seqs[0] < 0 && seqs[1] === seqs[0] + 1, "synthetic seqs must be contiguous");
});

test("two history batches never share synthetic seqs", () => {
  const first = mergeOlder([line(9)], [line(0, { db_id: 101 }), line(0, { db_id: 102 })], false);
  const second = mergeOlder(first, [line(0, { db_id: 99 }), line(0, { db_id: 100 })], false);

  const seqs = second.map((l) => l.seq);
  assert.equal(second.length, 5);
  assert.equal(new Set(seqs).size, seqs.length, "batches must not collide");
  assert.ok(seqs[0] < Math.min(...first.map((l) => l.seq)), "new batch goes below the oldest");
});

test("an already displayed db_id is not added twice", () => {
  const merged = mergeOlder([line(9, { db_id: 100 })], [line(0, { db_id: 100 })], false);
  assert.equal(merged.length, 1);
});

test("first load without an anchor dedups by content (ts may differ)", () => {
  const shown = [line(9, { ts: "2026-11-22T10:00:00Z", text: "hello" })];
  const fromDb = [line(0, { db_id: 55, ts: "2026-11-22T10:00:01Z", text: "hello" })];

  assert.equal(mergeOlder(shown, fromDb, true).length, 1, "loose dedup drops the copy");
  assert.equal(mergeOlder(shown, fromDb, false).length, 2, "anchored loads keep a repeated line");
});

test("empty batch is a no-op", () => {
  const prev = [line(9)];
  assert.equal(mergeOlder(prev, [], false), prev);
});

test("appendLive drops replayed seqs and keeps arrival order", () => {
  const prev = [line(1), line(2)];
  const merged = appendLive(prev, [line(2), line(3), line(4)]);

  assert.deepEqual(
    merged.map((l) => l.seq),
    [1, 2, 3, 4],
  );
});

test("appendLive returns prev when every frame is a duplicate", () => {
  const prev = [line(1), line(2)];
  assert.equal(appendLive(prev, [line(1), line(2)]), prev);
});

test("appendLive caps the window at the live size", () => {
  const prev = [line(1), line(2), line(3)];
  const merged = appendLive(prev, [line(4), line(5), line(6)], 4);

  assert.deepEqual(
    merged.map((l) => l.seq),
    [3, 4, 5, 6],
  );
});

test("appendLive no-op on empty incoming", () => {
  const prev = [line(1)];
  assert.equal(appendLive(prev, []), prev);
});
