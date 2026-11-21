const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");
const ts = require("typescript");

const compiled = ts.transpileModule(fs.readFileSync(path.join(__dirname, "update-hints.ts"), "utf8"), {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText;

assert.ok(!/\brequire\(/.test(compiled), "update-hints.ts must not pull runtime imports");

const mod = { exports: {} };
vm.runInNewContext(compiled, { exports: mod.exports, module: mod });
const { updateErrorHintKey } = mod.exports;

test("maps actionable failure kinds to their hint keys", () => {
  for (const [kind, want] of [
    ["network", "settings.retryUpdateNetwork"],
    ["rate_limited", "settings.updateErrorHintRateLimited"],
    ["security", "settings.updateErrorHintSecurity"],
  ]) {
    assert.equal(updateErrorHintKey(kind), want, kind);
  }
});

test("falls back to the generic hint for everything else", () => {
  for (const kind of ["not_found", "http", "invalid_response", "other", undefined]) {
    assert.equal(updateErrorHintKey(kind), "settings.updateErrorHintGeneric", String(kind));
  }
});
