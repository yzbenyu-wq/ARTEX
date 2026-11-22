"use client";
import * as React from "react";

import {
  BoxesIcon,
  DownloadIcon,
  PlayIcon,
  PlusIcon,
  RotateCcwIcon,
  SaveIcon,
  SearchIcon,
  Trash2Icon,
  UploadIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { useUITranslation } from "@/i18n/use-ui-translation";
import { api } from "@/lib/api";
import type { Agent, Tool, ToolPreset, ToolProxyMode, ToolStatus } from "@/lib/types";

// Traffic tools are host tools gated by the global 流量捕获 switch: bindable, but
// only usable when capture is on. Keep in sync with traffic.SeedToolMetas.
const TRAFFIC_TOOL_KEYS = new Set(["traffic_search", "traffic_get"]);

type ToolMetaState = {
  category: string;
  binary: string;
  proxyMode: ToolProxyMode;
  versionCmd: string;
  homepage: string;
};

const EMPTY_TOOL_META: ToolMetaState = {
  category: "",
  binary: "",
  proxyMode: "inherit",
  versionCmd: "",
  homepage: "",
};

function toolMetaFrom(tool: Tool): ToolMetaState {
  return {
    category: tool.category ?? "",
    binary: tool.binary ?? "",
    proxyMode: (tool.proxy_mode as ToolProxyMode | undefined) ?? "inherit",
    versionCmd: tool.version_cmd ?? "",
    homepage: tool.homepage ?? "",
  };
}

// ToolMetaFields is shared by the built-in and custom editors so both surfaces
// store the same toolbox metadata.
function ToolMetaFields({
  meta,
  onChange,
}: {
  meta: ToolMetaState;
  onChange: (patch: Partial<ToolMetaState>) => void;
}) {
  const { ui } = useUITranslation();
  return (
    <div className="grid gap-3 rounded-md border p-3">
      <div>
        <Label className="text-xs font-medium">{ui("screen.toolboxMeta")}</Label>
        <p className="text-muted-foreground text-[11px]">{ui("screen.toolboxMetaHint")}</p>
      </div>
      <div className="grid gap-2 sm:grid-cols-2">
        <div className="grid gap-1.5">
          <Label className="text-muted-foreground text-[11px]">{ui("screen.toolboxCategory")}</Label>
          <Input
            className="text-xs"
            placeholder="recon / web / vuln / utility"
            value={meta.category}
            onChange={(e) => onChange({ category: e.target.value })}
          />
        </div>
        <div className="grid gap-1.5">
          <Label className="text-muted-foreground text-[11px]">{ui("screen.toolboxBinary")}</Label>
          <Input
            className="font-mono text-xs"
            placeholder="httpx"
            value={meta.binary}
            onChange={(e) => onChange({ binary: e.target.value })}
          />
        </div>
        <div className="grid gap-1.5">
          <Label className="text-muted-foreground text-[11px]">{ui("screen.toolboxProxyMode")}</Label>
          <Select value={meta.proxyMode} onValueChange={(v) => onChange({ proxyMode: v as ToolProxyMode })}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {["inherit", "none", "http", "socks5", "env", "raw"].map((mode) => (
                <SelectItem key={mode} value={mode}>
                  {mode}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="grid gap-1.5">
          <Label className="text-muted-foreground text-[11px]">{ui("screen.toolboxVersionCmd")}</Label>
          <Input
            className="font-mono text-xs"
            placeholder="httpx -version"
            value={meta.versionCmd}
            onChange={(e) => onChange({ versionCmd: e.target.value })}
          />
        </div>
        <div className="grid gap-1.5 sm:col-span-2">
          <Label className="text-muted-foreground text-[11px]">{ui("screen.toolboxHomepage")}</Label>
          <Input
            className="text-xs"
            placeholder="https://github.com/projectdiscovery/httpx"
            value={meta.homepage}
            onChange={(e) => onChange({ homepage: e.target.value })}
          />
        </div>
      </div>
    </div>
  );
}

// paramRows flattens schema.properties into an ordered row list for editing.
// name/type/required are read-only (welded to the Go handler); description/default
// are editable. Non-scalar params (array/object) can't carry an editable default.
// parentKey is set for rows that live inside an array param's items.properties.
type ParamRow = {
  name: string;
  type: string;
  required: boolean;
  description: string;
  defaultStr: string; // "" = no default declared
  scalar: boolean;
  parentKey?: string; // set when this is a sub-field of an array param's items
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function toRows(schema: Record<string, any>): ParamRow[] {
  const props = (schema?.properties ?? {}) as Record<string, Record<string, unknown>>;
  const required = new Set<string>((schema?.required as string[]) ?? []);
  const rows: ParamRow[] = [];
  for (const [name, p] of Object.entries(props)) {
    const type = String(p?.type ?? "");
    const hasDefault = p != null && "default" in p && (p as Record<string, unknown>).default != null;
    rows.push({
      name,
      type,
      required: required.has(name),
      description: String(p?.description ?? ""),
      defaultStr: hasDefault ? String((p as Record<string, unknown>).default) : "",
      scalar: ["string", "integer", "number", "boolean"].includes(type),
    });
    // Expand items.properties for array params so sub-fields are editable.
    if (type === "array") {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const itemProps = (p as any)?.items?.properties as Record<string, Record<string, unknown>> | undefined;
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const itemRequired = new Set<string>(((p as any)?.items?.required as string[]) ?? []);
      if (itemProps) {
        for (const [subName, subP] of Object.entries(itemProps)) {
          const subType = String((subP as Record<string, unknown>)?.type ?? "");
          const subHasDefault = subP != null && "default" in subP && subP.default != null;
          rows.push({
            name: subName,
            type: subType,
            required: itemRequired.has(subName),
            description: String(subP?.description ?? ""),
            defaultStr: subHasDefault ? String(subP.default) : "",
            scalar: ["string", "integer", "number", "boolean"].includes(subType),
            parentKey: name,
          });
        }
      }
    }
  }
  return rows;
}

// coerceDefault turns the edited default string back into a typed JSON value, or
// undefined to drop the "default" key entirely.
function coerceDefault(type: string, raw: string): unknown {
  const s = raw.trim();
  if (s === "") return undefined;
  if (type === "integer" || type === "number") {
    const n = Number(s);
    return Number.isNaN(n) ? undefined : n;
  }
  if (type === "boolean") return s === "true";
  return raw;
}

// applyRows writes edited rows back into a deep-copied schema (structure untouched).
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function applyRows(schema: Record<string, any>, rows: ParamRow[]): Record<string, any> {
  const next = structuredClone(schema ?? {});
  const props = (next.properties ?? {}) as Record<string, Record<string, unknown>>;
  for (const r of rows) {
    if (r.parentKey) {
      // Sub-field of an array param's items.properties
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const parent = props[r.parentKey] as any;
      const subP = parent?.items?.properties?.[r.name] as Record<string, unknown> | undefined;
      if (!subP) continue;
      subP.description = r.description;
      if (r.scalar) {
        const dv = coerceDefault(r.type, r.defaultStr);
        if (dv === undefined) delete subP.default;
        else subP.default = dv;
      }
    } else {
      const p = props[r.name];
      if (!p) continue;
      p.description = r.description;
      if (r.scalar) {
        const dv = coerceDefault(r.type, r.defaultStr);
        if (dv === undefined) delete p.default;
        else p.default = dv;
      }
    }
  }
  return next;
}

// ToolEditor is the full edit form for one tool, rendered inside the drawer.
function ToolEditor({
  tool,
  agents,
  captureOn,
  onSaved,
  onClose,
}: {
  tool: Tool;
  agents: Agent[];
  captureOn: boolean;
  onSaved: () => void;
  onClose: () => void;
}) {
  const { ui } = useUITranslation();

  // traffic tools can't be bound/enabled until the global 流量捕获 switch is on.
  const trafficGated = TRAFFIC_TOOL_KEYS.has(tool.key) && !captureOn;
  const [description, setDescription] = React.useState(tool.description);
  const [bound, setBound] = React.useState<string[]>(tool.agents);
  const [enabled, setEnabled] = React.useState(tool.enabled);
  const [meta, setMeta] = React.useState<ToolMetaState>(() => toolMetaFrom(tool));
  const [rows, setRows] = React.useState<ParamRow[]>(() => toRows(tool.schema));
  const [saving, setSaving] = React.useState(false);

  const setRow = (i: number, patch: Partial<ParamRow>) =>
    setRows((rs) => rs.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const toggleAgent = (k: string) => setBound((b) => (b.includes(k) ? b.filter((x) => x !== k) : [...b, k]));

  async function save() {
    setSaving(true);
    try {
      await api.saveTool(tool.key, {
        description,
        schema: applyRows(tool.schema, rows),
        agents: bound,
        enabled,
        category: meta.category,
        binary: meta.binary,
        proxy_mode: meta.proxyMode,
        version_cmd: meta.versionCmd,
        homepage: meta.homepage,
      });
      toast.success(ui("screen.8465f58cd53be3", { v0: tool.key }));
      onSaved();
      onClose();
    } catch (e) {
      toast.error(ui("screen.6ade7baf8737c0") + (e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  async function reset() {
    try {
      await api.resetTool(tool.key);
      toast.success(ui("screen.7d4646712aedf3", { v0: tool.key }));
      onSaved();
      onClose();
    } catch (e) {
      toast.error(ui("screen.288bfb28a85177") + (e as Error).message);
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4">
        {trafficGated && (
          <div className="border-amber-500/40 bg-amber-500/10 text-muted-foreground rounded-md border px-3 py-2 text-xs">
            {ui("screen.3cb2efc9c9f048")}
            <b>{ui("screen.e81547323360ce")}</b>
            {ui("screen.f4aeb014f9325f")}
          </div>
        )}
        {/* binding + switch */}
        <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
          <div className="grid gap-1.5">
            <Label className="text-muted-foreground text-xs">{ui("screen.d2a8ed3e865d05")}</Label>
            <div className="flex flex-wrap gap-3">
              {agents.map((ag) => (
                <label key={ag.key} className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={bound.includes(ag.key)}
                    disabled={trafficGated}
                    onCheckedChange={() => toggleAgent(ag.key)}
                  />
                  {ag.name}
                  <span className="text-muted-foreground font-mono text-xs">{ag.key}</span>
                </label>
              ))}
              {agents.length === 0 && (
                <span className="text-muted-foreground text-xs">{ui("screen.6d026ee6e4dfc7")}</span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={setEnabled} id={`en-${tool.key}`} />
            <Label htmlFor={`en-${tool.key}`} className="text-sm">
              {ui("screen.f4f0ead1116b5b")}
            </Label>
          </div>
        </div>

        <ToolMetaFields meta={meta} onChange={(patch) => setMeta((m) => ({ ...m, ...patch }))} />

        {/* description */}
        <div className="grid gap-1.5">
          <Label className="text-muted-foreground text-xs">{ui("screen.9b967dfa2d272b")}</Label>
          <Textarea
            className="font-mono text-xs"
            rows={6}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>

        {/* params */}
        <div className="grid gap-2">
          <Label className="text-muted-foreground text-xs">{ui("screen.83e6de7c56800a")}</Label>
          {rows.length === 0 && <span className="text-muted-foreground text-xs">{ui("screen.c336c3a5f5baa0")}</span>}
          {rows.map((r, i) => (
            <div
              key={(r.parentKey ?? "") + "." + r.name}
              className={
                r.parentKey ? "border-l-2 border-muted ml-3 pl-3 grid gap-2 py-2" : "grid gap-2 rounded-md border p-3"
              }
            >
              <div className="flex flex-wrap items-center gap-2">
                {r.parentKey && <span className="text-muted-foreground font-mono text-[10px]">↳</span>}
                <span className="font-mono text-sm">{r.name}</span>
                <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
                  {r.type || "?"}
                </Badge>
                {r.required && (
                  <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                    {ui("screen.11da9dc4428537")}
                  </Badge>
                )}
                {r.parentKey && (
                  <span className="text-muted-foreground text-[10px]">{ui("screen.8f94ddf5a7c692")}</span>
                )}
              </div>
              <div className="grid gap-2">
                <div className="grid gap-1">
                  <Label className="text-muted-foreground text-[11px]">{ui("screen.dc2ba467fc7ac9")}</Label>
                  <Input
                    className="text-xs"
                    value={r.description}
                    onChange={(e) => setRow(i, { description: e.target.value })}
                  />
                </div>
                <div className="grid gap-1">
                  <Label className="text-muted-foreground text-[11px]">{ui("screen.306da767a461a5")}</Label>
                  <Input
                    className="text-xs"
                    placeholder={r.scalar ? ui("screen.74e8b9ae30845f") : ui("screen.1726133b7212ca")}
                    disabled={!r.scalar}
                    value={r.defaultStr}
                    onChange={(e) => setRow(i, { defaultStr: e.target.value })}
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      <Separator className="mt-4" />
      <div className="flex flex-wrap gap-2 p-4">
        <Button size="sm" onClick={save} disabled={saving}>
          <SaveIcon /> {ui("screen.a3030bf8f16dc6")}
        </Button>
        <Button size="sm" variant="outline" onClick={reset}>
          <RotateCcwIcon /> {ui("screen.ba2e93e73037c7")}
        </Button>
      </div>
    </div>
  );
}

// ToolGridCard is one clickable tile in the catalog grid.
function ToolGridCard({ tool, status, onClick }: { tool: Tool; status?: ToolStatus; onClick: () => void }) {
  const { ui } = useUITranslation();
  const binaryState = tool.binary
    ? status
      ? status.installed
        ? status.version || ui("screen.toolboxInstalled")
        : ui("screen.toolboxMissing")
      : ui("screen.toolboxChecking")
    : "";

  return (
    <button
      type="button"
      onClick={onClick}
      className="hover:border-primary/50 hover:bg-muted/40 focus-visible:ring-ring flex flex-col gap-2 rounded-lg border p-4 text-left transition-colors focus-visible:ring-2 focus-visible:outline-none"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-sm font-medium">{tool.key}</span>
        {tool.system ? (
          <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
            {ui("screen.5b50d7c4b5950d")}
          </Badge>
        ) : (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {ui("screen.31f80283afcce3")}
            {tool.kind}
          </Badge>
        )}
        {tool.source === "imported" && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {ui("screen.toolboxImported")}
          </Badge>
        )}
        {tool.source === "preset" && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {ui("screen.toolboxPresetSource")}
          </Badge>
        )}
        {tool.category && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            {tool.category}
          </Badge>
        )}
        {tool.deferred && (
          <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
            deferred
          </Badge>
        )}
        {!tool.enabled && (
          <Badge variant="outline" className="text-destructive px-1.5 py-0 text-[10px]">
            {ui("screen.a8c3698b5b8c48")}
          </Badge>
        )}
        <Badge
          variant="secondary"
          className="ml-auto px-1.5 py-0 text-[10px] tabular-nums"
          title={ui("screen.f6304be61c53f3", { v0: tool.calls ?? 0 })}
        >
          {ui("screen.653b123c956d3b")}
          {tool.calls ?? 0}
        </Badge>
      </div>
      <p className="text-muted-foreground line-clamp-1 h-4 text-xs">
        {tool.description || ui("screen.0042f188770453")}
      </p>
      {tool.binary && (
        <div className="flex flex-wrap items-center gap-1">
          <Badge
            variant={status?.installed ? "secondary" : "outline"}
            className={"px-1.5 py-0 font-mono text-[10px]" + (status && !status.installed ? " text-destructive" : "")}
          >
            {tool.binary} · {binaryState}
          </Badge>
          {tool.proxy_mode && tool.proxy_mode !== "inherit" && (
            <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
              {ui("screen.5e84ea61e8386a")}: {tool.proxy_mode}
            </Badge>
          )}
        </div>
      )}
      <div className="mt-auto flex flex-wrap gap-1 pt-1">
        {tool.agents.length === 0 && (
          <span className="text-muted-foreground text-[10px]">{ui("screen.7d97c77ac3183d")}</span>
        )}
        {tool.agents.map((a) => (
          <Badge key={a} variant="outline" className="px-1.5 py-0 text-[10px]">
            {a}
          </Badge>
        ))}
      </div>
    </button>
  );
}

export default function ToolsPage() {
  const { ui } = useUITranslation();

  const [tools, setTools] = React.useState<Tool[]>([]);
  const [agents, setAgents] = React.useState<Agent[]>([]);
  const [statuses, setStatuses] = React.useState<Record<string, ToolStatus>>({});
  const [captureOn, setCaptureOn] = React.useState(false);
  const [selectedKey, setSelectedKey] = React.useState<string | null>(null);
  const [customEdit, setCustomEdit] = React.useState<Tool | "new" | null>(null);
  const [presetsOpen, setPresetsOpen] = React.useState(false);
  const [importing, setImporting] = React.useState(false);
  const [checking, setChecking] = React.useState(false);
  const fileRef = React.useRef<HTMLInputElement>(null);

  const loadStatuses = React.useCallback((withVersions = false) => {
    api
      .toolStatus(withVersions)
      .then((rows) => {
        const next: Record<string, ToolStatus> = {};
        for (const row of rows) next[row.key] = row;
        setStatuses(next);
      })
      .catch(() => setStatuses({}));
  }, []);

  const reload = React.useCallback(() => {
    api
      .tools()
      .then(setTools)
      .catch(() => setTools([]));
    loadStatuses();
  }, [loadStatuses]);
  React.useEffect(() => {
    reload();
    api
      .agents()
      .then(setAgents)
      .catch(() => {});
    api
      .settings()
      .then((s) => setCaptureOn(!!s.traffic_capture))
      .catch(() => {});
  }, [reload]);

  const [query, setQuery] = React.useState("");

  async function checkVersions() {
    setChecking(true);
    try {
      await api.toolStatus(true).then((rows) => {
        const next: Record<string, ToolStatus> = {};
        for (const row of rows) next[row.key] = row;
        setStatuses(next);
      });
      toast.success(ui("screen.toolboxVersionsChecked"));
    } catch (e) {
      toast.error(ui("screen.toolboxStatusFailed") + (e as Error).message);
    } finally {
      setChecking(false);
    }
  }

  async function exportTools() {
    try {
      const data = await api.exportTools();
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `artex-tools-${new Date().toISOString().slice(0, 10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      toast.error(ui("screen.toolboxExportFailed") + (e as Error).message);
    }
  }

  async function importFile(file: File) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(await file.text());
    } catch {
      toast.error(ui("screen.toolboxInvalidJson"));
      return;
    }
    const list = Array.isArray(parsed) ? parsed : (parsed as { tools?: unknown } | null)?.tools;
    if (!Array.isArray(list)) {
      toast.error(ui("screen.toolboxInvalidJson"));
      return;
    }
    setImporting(true);
    try {
      const result = await api.importTools(list as Partial<Tool>[]);
      const detail = result.errors.length ? " " + result.errors.slice(0, 3).join("; ") : "";
      toast.success(ui("screen.toolboxImportDone", { v0: result.imported, v1: result.skipped }) + detail);
      reload();
    } catch (e) {
      toast.error(ui("screen.toolboxImportFailed") + (e as Error).message);
    } finally {
      setImporting(false);
    }
  }

  const selected = tools.find((t) => t.key === selectedKey) ?? null;
  const matchTool = React.useCallback(
    (t: Tool) => {
      const q = query.trim().toLowerCase();
      if (!q) return true;
      return (
        t.key.toLowerCase().includes(q) ||
        t.description.toLowerCase().includes(q) ||
        t.agents.some((a) => a.toLowerCase().includes(q))
      );
    },
    [query],
  );
  const systemTools = tools.filter((t) => t.system && matchTool(t));
  const customTools = tools.filter((t) => !t.system && matchTool(t));
  const allSystemCount = tools.filter((t) => t.system).length;
  const allCustomCount = tools.filter((t) => !t.system).length;
  // clicking a built-in tool opens the binding drawer; a custom tool opens its full editor.
  const openTool = (t: Tool) => (t.system ? setSelectedKey(t.key) : setCustomEdit(t));

  return (
    <div className="flex flex-1 flex-col gap-4 md:gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">{ui("screen.toolboxTitle")}</h1>
          <p className="text-muted-foreground text-sm">{ui("screen.070c5bf8a7a0cb")}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative w-64">
            <SearchIcon className="text-muted-foreground absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2" />
            <Input
              className="h-8 pl-8 text-sm"
              placeholder={ui("screen.2b7665ed985e0a")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <input
            ref={fileRef}
            type="file"
            accept=".json,application/json"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) void importFile(file);
              e.target.value = "";
            }}
          />
          <Button size="sm" variant="outline" onClick={() => setPresetsOpen(true)}>
            <BoxesIcon /> {ui("screen.toolboxPresets")}
          </Button>
          <Button size="sm" variant="outline" onClick={() => fileRef.current?.click()} disabled={importing}>
            <UploadIcon /> {ui("screen.toolboxImport")}
          </Button>
          <Button size="sm" variant="outline" onClick={() => void exportTools()}>
            <DownloadIcon /> {ui("screen.toolboxExport")}
          </Button>
          <Button size="sm" variant="outline" onClick={() => void checkVersions()} disabled={checking}>
            <RotateCcwIcon /> {checking ? ui("screen.toolboxChecking") : ui("screen.toolboxCheckVersions")}
          </Button>
        </div>
      </div>

      <Tabs defaultValue="system">
        <TabsList>
          <TabsTrigger value="system">{ui("screen.6ea702ad9c2b49")}</TabsTrigger>
          <TabsTrigger value="custom">{ui("screen.1e976a626f6f83")}</TabsTrigger>
        </TabsList>

        <TabsContent value="system">
          <Card>
            <CardHeader>
              <CardTitle>{ui("screen.6ea702ad9c2b49")}</CardTitle>
              <CardDescription>
                {query.trim()
                  ? ui("screen.4d12f613be8bf8", { v0: systemTools.length, v1: allSystemCount })
                  : ui("screen.f2d63a71fa4dfb", { v0: allSystemCount })}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {systemTools.length === 0 ? (
                <p className="text-muted-foreground py-6 text-center text-sm">
                  {query.trim() ? ui("screen.c31006ada9d69d") : ui("screen.20e14c773f53f6")}
                </p>
              ) : (
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  {systemTools.map((t) => (
                    <ToolGridCard key={t.key} tool={t} status={statuses[t.key]} onClick={() => openTool(t)} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="custom">
          <Card>
            <CardHeader>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <CardTitle>{ui("screen.1e976a626f6f83")}</CardTitle>
                  <CardDescription>
                    {query.trim()
                      ? ui("screen.c596c5a80767be", { v0: customTools.length, v1: allCustomCount })
                      : ui("screen.431df6fd85c9b3", { v0: allCustomCount })}
                  </CardDescription>
                </div>
                <Button size="sm" onClick={() => setCustomEdit("new")}>
                  <PlusIcon /> {ui("screen.87b2833f370844")}
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {customTools.length === 0 ? (
                <p className="text-muted-foreground py-6 text-center text-sm">
                  {query.trim() ? ui("screen.918b9515dae7aa") : ui("screen.b2ff462a0cff72")}
                </p>
              ) : (
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  {customTools.map((t) => (
                    <ToolGridCard key={t.key} tool={t} status={statuses[t.key]} onClick={() => openTool(t)} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <Sheet open={!!selected} onOpenChange={(o) => !o && setSelectedKey(null)}>
        <SheetContent side="right" className="gap-0 p-0 data-[side=right]:w-[30vw] data-[side=right]:sm:max-w-[30vw]">
          {selected && (
            <>
              <SheetHeader className="px-4">
                <SheetTitle className="font-mono">{selected.key}</SheetTitle>
                <SheetDescription>{ui("screen.e8bbb2829a4b94")}</SheetDescription>
              </SheetHeader>
              <ToolEditor
                key={selected.key}
                tool={selected}
                agents={agents}
                captureOn={captureOn}
                onSaved={reload}
                onClose={() => setSelectedKey(null)}
              />
            </>
          )}
        </SheetContent>
      </Sheet>

      <CustomToolDialog
        edit={customEdit}
        agents={agents}
        onClose={() => setCustomEdit(null)}
        onSaved={() => {
          setCustomEdit(null);
          reload();
        }}
      />

      <PresetPicker
        open={presetsOpen}
        onClose={() => setPresetsOpen(false)}
        onInstalled={() => {
          setPresetsOpen(false);
          reload();
        }}
      />
    </div>
  );
}

// ---- 自定义工具编辑器 ----

type CustomKind = "shell" | "command" | "script" | "http" | "recipe";

type ExecState = {
  command: string;
  code: string;
  method: string;
  url: string;
  headers: string;
  body: string;
  timeout_ms: string;
  proxy: string;
  use_recording_proxy: boolean;
  argv: string;
  stdin: string;
  allowed_exit_codes: string;
};

const EMPTY_EXEC_STATE: ExecState = {
  command: "",
  code: "",
  method: "GET",
  url: "",
  headers: "",
  body: "",
  timeout_ms: "",
  proxy: "",
  use_recording_proxy: false,
  argv: "",
  stdin: "",
  allowed_exit_codes: "",
};

function CustomToolDialog({
  edit,
  agents,
  onClose,
  onSaved,
}: {
  edit: Tool | "new" | null;
  agents: Agent[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const { ui } = useUITranslation();

  const isNew = edit === "new";
  const tool = edit && edit !== "new" ? edit : null;
  const [key, setKey] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [kind, setKind] = React.useState<CustomKind>("shell");
  const [meta, setMeta] = React.useState<ToolMetaState>(EMPTY_TOOL_META);
  const [ex, setEx] = React.useState<ExecState>(EMPTY_EXEC_STATE);
  const [schemaText, setSchemaText] = React.useState("");
  const [bound, setBound] = React.useState<string[]>([]);
  const [deferred, setDeferred] = React.useState(false);
  const [enabled, setEnabled] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [paramsText, setParamsText] = React.useState("");
  const [testing, setTesting] = React.useState(false);
  const [testResult, setTestResult] = React.useState<{ output: string; is_error: boolean } | null>(null);

  // (re)load form state when opening.
  React.useEffect(() => {
    if (!edit) return;
    setParamsText("");
    setTestResult(null);
    if (edit === "new") {
      setKey("");
      setDescription("");
      setKind("shell");
      setMeta(EMPTY_TOOL_META);
      setEx(EMPTY_EXEC_STATE);
      setSchemaText("");
      setBound([]);
      setDeferred(false);
      setEnabled(true);
      return;
    }
    const t = edit;
    const e = (t.exec ?? {}) as Record<string, unknown>;
    setKey(t.key);
    setDescription(t.description);
    setKind((t.kind as CustomKind) ?? "shell");
    setMeta(toolMetaFrom(t));
    setEx({
      command: String(e.command ?? ""),
      code: String(e.code ?? ""),
      method: String(e.method ?? "GET"),
      url: String(e.url ?? ""),
      headers: e.headers ? JSON.stringify(e.headers, null, 2) : "",
      body: String(e.body ?? ""),
      timeout_ms: e.timeout_ms ? String(e.timeout_ms) : "",
      proxy: String(e.proxy ?? ""),
      use_recording_proxy: !!e.use_recording_proxy,
      argv: e.argv ? JSON.stringify(e.argv, null, 2) : "",
      stdin: String(e.stdin ?? ""),
      allowed_exit_codes: Array.isArray(e.allowed_exit_codes) ? (e.allowed_exit_codes as number[]).join(",") : "",
    });
    setSchemaText(t.schema && Object.keys(t.schema).length ? JSON.stringify(t.schema, null, 2) : "");
    setBound(t.agents ?? []);
    setDeferred(!!t.deferred);
    setEnabled(t.enabled);
  }, [edit]);

  const toggleAgent = (k: string) => setBound((b) => (b.includes(k) ? b.filter((x) => x !== k) : [...b, k]));

  // recipe 表单校验：argv 必须是非空 JSON 数组，退出码必须全是整数。
  function parseRecipeArgv(): unknown[] | null {
    if (!ex.argv.trim()) return null;
    try {
      const v = JSON.parse(ex.argv);
      return Array.isArray(v) && v.length > 0 ? v : null;
    } catch {
      return null;
    }
  }
  function parseRecipeCodes(): number[] | null {
    const out: number[] = [];
    for (const t of ex.allowed_exit_codes.split(",").map((s) => s.trim()).filter(Boolean)) {
      if (!/^\d+$/.test(t)) return null;
      out.push(Number(t));
    }
    return out;
  }

  function buildExec(): Record<string, unknown> {
    if (kind === "shell") return {};
    const t = ex.timeout_ms ? Number(ex.timeout_ms) : undefined;
    if (kind === "command") return { command: ex.command, timeout_ms: t };
    if (kind === "script") return { code: ex.code, timeout_ms: t };
    if (kind === "recipe") {
      return {
        argv: parseRecipeArgv() ?? [],
        stdin: ex.stdin,
        timeout_ms: t,
        allowed_exit_codes: parseRecipeCodes() ?? [],
      };
    }
    let headers: Record<string, string> = {};
    if (ex.headers.trim()) {
      try {
        headers = JSON.parse(ex.headers);
      } catch {
        /* validated on save */
      }
    }
    return {
      method: ex.method,
      url: ex.url,
      headers,
      body: ex.body,
      timeout_ms: t,
      proxy: ex.proxy,
      use_recording_proxy: ex.use_recording_proxy,
    };
  }

  async function save() {
    if (isNew && !/^[a-z][a-z0-9_]*$/.test(key.trim())) {
      toast.error(ui("screen.c642001d5315f6"));
      return;
    }
    let schema: Record<string, unknown> = {};
    if (schemaText.trim()) {
      try {
        schema = JSON.parse(schemaText);
      } catch {
        toast.error(ui("screen.bab199cfb603cf"));
        return;
      }
    }
    if (kind === "http" && ex.headers.trim()) {
      try {
        JSON.parse(ex.headers);
      } catch {
        toast.error(ui("screen.fa9c8243f5a643"));
        return;
      }
    }
    if (kind === "http") {
      const props = (schema as { properties?: Record<string, unknown> }).properties;
      if (!props || Object.keys(props).length === 0) {
        toast.error(ui("screen.0ba562d5560cea"));
        return;
      }
    }
    if (kind === "recipe") {
      const props = (schema as { properties?: Record<string, unknown> }).properties;
      if (!props || Object.keys(props).length === 0) {
        toast.error(ui("screen.toolboxRecipeSchemaRequired"));
        return;
      }
      if (!parseRecipeArgv()) {
        toast.error(ui("screen.toolboxRecipeInvalidArgv"));
        return;
      }
      if (!parseRecipeCodes()) {
        toast.error(ui("screen.toolboxRecipeInvalidExit"));
        return;
      }
    }
    setSaving(true);
    const payload = {
      description,
      schema: kind === "shell" ? {} : schema,
      agents: bound,
      enabled,
      kind,
      exec: buildExec(),
      deferred: kind === "shell" ? false : deferred,
      category: meta.category,
      binary: meta.binary,
      proxy_mode: meta.proxyMode,
      version_cmd: meta.versionCmd,
      homepage: meta.homepage,
    };
    try {
      if (isNew) await api.createCustomTool({ key: key.trim(), ...payload });
      else await api.updateCustomTool(tool!.key, payload);
      toast.success(isNew ? ui("screen.53264be8bfee72") : ui("screen.1bd91a7d0c53c9"));
      onSaved();
    } catch (e) {
      toast.error(ui("screen.6ade7baf8737c0") + (e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  async function del() {
    if (!tool) return;
    try {
      await api.deleteCustomTool(tool.key);
      toast.success(ui("screen.077a6d37719a0e"));
      onSaved();
    } catch (e) {
      toast.error(ui("screen.0580fcee37e020") + (e as Error).message);
    }
  }
  // runTest dry-runs the CURRENT form (unsaved) with the sample params, so a
  // template/script/request can be debugged before saving.
  async function runTest() {
    let params: Record<string, unknown> = {};
    if (paramsText.trim()) {
      try {
        params = JSON.parse(paramsText);
      } catch {
        toast.error(ui("screen.5194c511368a9c"));
        return;
      }
    }
    if (kind === "http" && ex.headers.trim()) {
      try {
        JSON.parse(ex.headers);
      } catch {
        toast.error(ui("screen.fa9c8243f5a643"));
        return;
      }
    }
    if (kind === "recipe") {
      if (!parseRecipeArgv()) {
        toast.error(ui("screen.toolboxRecipeInvalidArgv"));
        return;
      }
      if (!parseRecipeCodes()) {
        toast.error(ui("screen.toolboxRecipeInvalidExit"));
        return;
      }
    }
    setTesting(true);
    setTestResult(null);
    try {
      const r = await api.testCustomTool({
        kind,
        exec: buildExec(),
        params,
        proxy_mode: meta.proxyMode,
        binary: meta.binary,
      });
      setTestResult(r);
    } catch (e) {
      setTestResult({ output: (e as Error).message, is_error: true });
    } finally {
      setTesting(false);
    }
  }

  return (
    <Sheet open={!!edit} onOpenChange={(o) => !o && onClose()}>
      <SheetContent
        side="right"
        className="flex flex-col gap-0 p-0 data-[side=right]:w-[45vw] data-[side=right]:sm:max-w-[45vw] data-[side=right]:min-w-[480px]"
      >
        <SheetHeader className="px-4">
          <SheetTitle>
            {isNew ? ui("screen.87b2833f370844") : ui("screen.f6de91250ef4ad", { v0: tool?.key })}
          </SheetTitle>
          <SheetDescription>{ui("screen.5aedc801135f87")}</SheetDescription>
        </SheetHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-4 pb-4">
          <div className="grid gap-1.5">
            <Label className="text-xs">Key</Label>
            <Input
              className="font-mono"
              placeholder={ui("screen.9210ea3a655465")}
              value={key}
              disabled={!isNew}
              onChange={(e) => setKey(e.target.value)}
            />
          </div>
          <div className="grid gap-1.5">
            <Label className="text-xs">{ui("screen.e4c430635064b5")}</Label>
            <Textarea rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>

          <div className="grid gap-1.5">
            <Label className="text-xs">{ui("screen.ba40014ff496f6")}</Label>
            <Select value={kind} onValueChange={(v) => setKind(v as CustomKind)}>
              <SelectTrigger className="w-56">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="shell">{ui("screen.896f9cb6fa7276")}</SelectItem>
                <SelectItem value="command">{ui("screen.fcf63aa2fdd22f")}</SelectItem>
                <SelectItem value="script">{ui("screen.2a97d1c87f3d4d")}</SelectItem>
                <SelectItem value="http">{ui("screen.f49ce697cf29c0")}</SelectItem>
                <SelectItem value="recipe">{ui("screen.toolboxKindRecipe")}</SelectItem>
              </SelectContent>
            </Select>
            {kind === "shell" && <p className="text-muted-foreground text-xs">{ui("screen.d8bcec9f7cd637")}</p>}
          </div>

          <ToolMetaFields meta={meta} onChange={(patch) => setMeta((m) => ({ ...m, ...patch }))} />

          {kind === "command" && (
            <div className="grid gap-1.5">
              <Label className="text-xs">
                {ui("screen.2d5caa25ce29ff")}
                {"{param}"}
                {ui("screen.e9f1af3c3757f9")}
                {"{ports}"} {"{target}"}
                {ui("punctuation.f2e44c6c95")}
              </Label>
              <Textarea
                className="font-mono text-xs"
                rows={2}
                value={ex.command}
                onChange={(e) => setEx({ ...ex, command: e.target.value })}
              />
            </div>
          )}
          {kind === "script" && (
            <div className="grid gap-1.5">
              <Label className="text-xs">{ui("screen.95acd4249dea97")}</Label>
              <Textarea
                className="font-mono text-xs"
                rows={10}
                value={ex.code}
                placeholder={"import json,sys\nargs=json.load(sys.stdin)\nprint(...)"}
                onChange={(e) => setEx({ ...ex, code: e.target.value })}
              />
            </div>
          )}
          {kind === "http" && (
            <div className="grid gap-2">
              <div className="flex gap-2">
                <div className="grid gap-1.5">
                  <Label className="text-xs">Method</Label>
                  <Input
                    className="w-24"
                    value={ex.method}
                    onChange={(e) => setEx({ ...ex, method: e.target.value })}
                  />
                </div>
                <div className="grid flex-1 gap-1.5">
                  <Label className="text-xs">
                    {ui("screen.69101230b8f993")}
                    {"{param}"}
                    {ui("punctuation.f2e44c6c95")}
                  </Label>
                  <Input
                    className="font-mono text-xs"
                    value={ex.url}
                    onChange={(e) => setEx({ ...ex, url: e.target.value })}
                  />
                </div>
              </div>
              <div className="grid gap-1.5">
                <Label className="text-xs">
                  {ui("screen.279bfc6d553f63")}
                  {"{param}"}
                  {ui("punctuation.f2e44c6c95")}
                </Label>
                <Textarea
                  className="font-mono text-xs"
                  rows={2}
                  value={ex.headers}
                  placeholder={'{"Authorization": "Bearer {token}"}'}
                  onChange={(e) => setEx({ ...ex, headers: e.target.value })}
                />
              </div>
              <div className="grid gap-1.5">
                <Label className="text-xs">
                  {ui("screen.7a58b49ecafbfe")}
                  {"{param}"}
                  {ui("punctuation.f2e44c6c95")}
                </Label>
                <Textarea
                  className="font-mono text-xs"
                  rows={2}
                  value={ex.body}
                  onChange={(e) => setEx({ ...ex, body: e.target.value })}
                />
              </div>
              <div className="flex items-center gap-4">
                <div className="grid gap-1.5">
                  <Label className="text-xs">{ui("screen.17b9f1c37b2057")}</Label>
                  <Input
                    className="font-mono text-xs w-56"
                    value={ex.proxy}
                    onChange={(e) => setEx({ ...ex, proxy: e.target.value })}
                  />
                </div>
                <label className="mt-4 flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={ex.use_recording_proxy}
                    onCheckedChange={(v) => setEx({ ...ex, use_recording_proxy: !!v })}
                  />
                  {ui("screen.5bd60af132eb13")}
                </label>
              </div>
            </div>
          )}

          {kind === "recipe" && (
            <div className="grid gap-2">
              <div className="grid gap-1.5">
                <Label className="text-xs">{ui("screen.toolboxRecipeArgv")}</Label>
                <Textarea
                  className="font-mono text-xs"
                  rows={7}
                  value={ex.argv}
                  placeholder={'[\n  ["-silent"],\n  ["-u", "{target}"],\n  ["-p", "{ports}"],\n  ["-proxy", "{proxy}"]\n]'}
                  onChange={(e) => setEx({ ...ex, argv: e.target.value })}
                />
                <p className="text-muted-foreground text-xs">{ui("screen.toolboxRecipeArgvHint")}</p>
              </div>
              <div className="grid gap-1.5">
                <Label className="text-xs">{ui("screen.toolboxRecipeStdin")}</Label>
                <Textarea
                  className="font-mono text-xs"
                  rows={2}
                  value={ex.stdin}
                  onChange={(e) => setEx({ ...ex, stdin: e.target.value })}
                />
              </div>
              <div className="grid gap-1.5">
                <Label className="text-xs">{ui("screen.toolboxRecipeAllowedExit")}</Label>
                <Input
                  className="font-mono text-xs w-40"
                  placeholder="0"
                  value={ex.allowed_exit_codes}
                  onChange={(e) => setEx({ ...ex, allowed_exit_codes: e.target.value })}
                />
              </div>
            </div>
          )}

          {kind !== "shell" && (
            <div className="flex items-center gap-3">
              <div className="grid gap-1.5">
                <Label className="text-xs">{ui("screen.38b6f0e56d09c4")}</Label>
                <Input
                  type="number"
                  className="w-32"
                  value={ex.timeout_ms}
                  onChange={(e) => setEx({ ...ex, timeout_ms: e.target.value })}
                />
              </div>
            </div>
          )}

          {kind !== "shell" && (
            <div className="grid gap-1.5">
              <Label className="text-xs">
                {ui("screen.7ff6a7d0a41f31")}
                {kind === "http"
                  ? ui("screen.46f4df9c0d2da9")
                  : kind === "recipe"
                    ? ui("screen.toolboxRecipeSchemaHint")
                    : ui("screen.84780d91a21684")}
              </Label>
              <Textarea
                className="font-mono text-xs"
                rows={4}
                value={schemaText}
                placeholder={'{"type":"object","properties":{"target":{"type":"string"}},"required":["target"]}'}
                onChange={(e) => setSchemaText(e.target.value)}
              />
            </div>
          )}

          <div className="grid gap-1.5">
            <Label className="text-muted-foreground text-xs">{ui("screen.7bb739274d9847")}</Label>
            <div className="flex flex-wrap gap-3">
              {agents.map((a) => (
                <label key={a.key} className="flex items-center gap-2 text-sm">
                  <Checkbox checked={bound.includes(a.key)} onCheckedChange={() => toggleAgent(a.key)} />
                  {a.name}
                  <span className="text-muted-foreground font-mono text-xs">{a.key}</span>
                </label>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-6">
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={enabled} onCheckedChange={setEnabled} /> {ui("screen.f4f0ead1116b5b")}
            </label>
            {kind !== "shell" && (
              <label className="flex items-center gap-2 text-sm">
                <Switch checked={deferred} onCheckedChange={setDeferred} /> {ui("screen.73edf6b3638e54")}
              </label>
            )}
          </div>

          {kind !== "shell" && (
            <div className="grid gap-1.5 rounded-md border p-3">
              <Label className="text-xs font-medium">{ui("screen.609b160d85ef96")}</Label>
              <Textarea
                className="font-mono text-xs"
                rows={2}
                value={paramsText}
                placeholder={ui("screen.4e9ddebfaff607")}
                onChange={(e) => setParamsText(e.target.value)}
              />
              <div>
                <Button size="sm" variant="outline" onClick={runTest} disabled={testing}>
                  <PlayIcon /> {testing ? ui("screen.14d9f2d830adb2") : ui("screen.1bb75d7ff2bf84")}
                </Button>
              </div>
              {testResult && (
                <pre
                  className={
                    "max-h-64 overflow-auto whitespace-pre-wrap break-words rounded bg-muted p-2 font-mono text-xs " +
                    (testResult.is_error ? "text-destructive" : "")
                  }
                >
                  {testResult.output || ui("screen.8ae64c2c8ce582")}
                </pre>
              )}
            </div>
          )}
        </div>

        <Separator />
        <div className="flex items-center gap-2 p-4">
          <Button size="sm" onClick={save} disabled={saving}>
            <SaveIcon /> {isNew ? ui("screen.cde2cd071d25bb") : ui("screen.a3030bf8f16dc6")}
          </Button>
          {!isNew && (
            <Button size="sm" variant="outline" className="text-destructive" onClick={del}>
              <Trash2Icon /> {ui("screen.2f9daa828907b9")}
            </Button>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

// PresetPicker installs recipes from the server's embedded catalog. Installed
// presets arrive disabled + deferred with no Agent bindings, so enabling and
// binding stays an explicit per-tool decision here in the toolbox.
function PresetPicker({
  open,
  onClose,
  onInstalled,
}: {
  open: boolean;
  onClose: () => void;
  onInstalled: () => void;
}) {
  const { ui } = useUITranslation();
  const [presets, setPresets] = React.useState<ToolPreset[]>([]);
  const [selected, setSelected] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [installing, setInstalling] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    setLoading(true);
    setSelected([]);
    api
      .listToolPresets()
      .then((rows) => {
        setPresets(rows);
        setSelected(rows.filter((p) => !p.installed).map((p) => p.key));
      })
      .catch((e) => toast.error(ui("screen.toolboxPresetsFailed") + (e as Error).message))
      .finally(() => setLoading(false));
  }, [open, ui]);

  const toggle = (k: string) => setSelected((s) => (s.includes(k) ? s.filter((x) => x !== k) : [...s, k]));

  async function install() {
    if (selected.length === 0) return;
    setInstalling(true);
    try {
      const r = await api.installToolPresets(selected);
      const detail = r.errors.length ? " " + r.errors.slice(0, 3).join("; ") : "";
      toast.success(ui("screen.toolboxPresetInstallDone", { v0: r.installed.length, v1: r.skipped.length }) + detail);
      onInstalled();
    } catch (e) {
      toast.error(ui("screen.toolboxPresetInstallFailed") + (e as Error).message);
    } finally {
      setInstalling(false);
    }
  }

  return (
    <Sheet open={open} onOpenChange={(o) => !o && onClose()}>
      <SheetContent
        side="right"
        className="flex flex-col gap-0 p-0 data-[side=right]:w-[38vw] data-[side=right]:sm:max-w-[38vw] data-[side=right]:min-w-[440px]"
      >
        <SheetHeader className="px-4">
          <SheetTitle>{ui("screen.toolboxPresets")}</SheetTitle>
          <SheetDescription>{ui("screen.toolboxPresetsHint")}</SheetDescription>
        </SheetHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-4 py-3">
          {loading ? (
            <p className="text-muted-foreground py-6 text-center text-sm">{ui("screen.toolboxChecking")}</p>
          ) : presets.length === 0 ? (
            <p className="text-muted-foreground py-6 text-center text-sm">{ui("screen.toolboxPresetNone")}</p>
          ) : (
            presets.map((p) => (
              <label
                key={p.key}
                className={
                  "flex items-start gap-2 rounded-md border p-3 text-sm " +
                  (p.installed ? "opacity-60" : "hover:border-primary/50 cursor-pointer")
                }
              >
                <Checkbox
                  className="mt-0.5"
                  checked={p.installed || selected.includes(p.key)}
                  disabled={p.installed}
                  onCheckedChange={() => toggle(p.key)}
                />
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="flex flex-wrap items-center gap-1.5">
                    <span className="font-mono text-sm font-medium">{p.key}</span>
                    {p.category && (
                      <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                        {p.category}
                      </Badge>
                    )}
                    {p.binary && (
                      <Badge variant="secondary" className="px-1.5 py-0 font-mono text-[10px]">
                        {p.binary}
                      </Badge>
                    )}
                    {p.proxy_mode !== "inherit" && (
                      <Badge variant="outline" className="px-1.5 py-0 text-[10px]">
                        {ui("screen.5e84ea61e8386a")}: {p.proxy_mode}
                      </Badge>
                    )}
                    {p.installed && (
                      <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
                        {ui("screen.toolboxInstalled")}
                      </Badge>
                    )}
                  </span>
                  <span className="text-muted-foreground text-xs">{p.description}</span>
                </span>
              </label>
            ))
          )}
        </div>

        <Separator />
        <div className="flex items-center gap-2 p-4">
          <Button size="sm" onClick={install} disabled={installing || selected.length === 0}>
            <PlusIcon /> {ui("screen.toolboxPresetInstall")}
          </Button>
          <Button size="sm" variant="outline" onClick={onClose}>
            {ui("screen.2cd0f3be8738a8")}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  );
}
