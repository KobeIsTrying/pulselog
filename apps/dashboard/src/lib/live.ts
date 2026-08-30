import type { LogEvent, OverviewStats } from "./types";

export type LiveStatus = "live" | "reconnecting" | "disconnected";

export interface LiveFilter {
  projectId?: string;
  service?: string;
  level?: string;
  q?: string;
  eventId?: string;
}

export interface LiveEnvelope {
  v?: number;
  type?: string;
  project_id?: string;
  data?: {
    event_id?: string;
    project_id?: string;
    service?: string;
    level?: string;
    message?: string;
    timestamp?: string;
    host?: string;
    trace_id?: string;
    metadata?: Record<string, string>;
  };
}

export function matchesLiveFilter(ev: LogEvent, f: LiveFilter): boolean {
  if (f.projectId && ev.project_id && ev.project_id !== f.projectId) return false;
  if (f.service && ev.service !== f.service) return false;
  if (f.level && ev.level !== f.level) return false;
  if (f.eventId && ev.event_id !== f.eventId) return false;
  if (f.q && !ev.message.toLowerCase().includes(f.q.toLowerCase())) return false;
  return true;
}

export function mergeLiveLogs(existing: LogEvent[], incoming: LogEvent[], cap = 200): LogEvent[] {
  const seen = new Set(existing.map((e) => e.event_id));
  const add: LogEvent[] = [];
  for (const ev of incoming) {
    if (!ev.event_id || seen.has(ev.event_id)) continue;
    seen.add(ev.event_id);
    add.push(ev);
  }
  return [...add, ...existing].slice(0, cap);
}

export function envelopeToLog(env: LiveEnvelope): LogEvent | null {
  if (env.type !== "log.created" || !env.data?.event_id) return null;
  return {
    event_id: env.data.event_id,
    project_id: env.data.project_id,
    timestamp: env.data.timestamp || new Date().toISOString(),
    ingested_at: env.data.timestamp || new Date().toISOString(),
    service: env.data.service || "",
    level: env.data.level || "INFO",
    message: env.data.message || "",
    host: env.data.host,
    trace_id: env.data.trace_id,
    metadata: env.data.metadata,
  };
}

export function parseLiveFrame(raw: string): LogEvent | null {
  try {
    return envelopeToLog(JSON.parse(raw) as LiveEnvelope);
  } catch {
    return null;
  }
}

export function bumpOverview(ov: OverviewStats, events: LogEvent[]): OverviewStats {
  const next = { ...ov };
  for (const e of events) {
    next.total += 1;
    switch (e.level.toUpperCase()) {
      case "ERROR":
        next.error += 1;
        break;
      case "FATAL":
        next.fatal += 1;
        break;
      case "WARN":
        next.warn += 1;
        break;
      case "INFO":
        next.info += 1;
        break;
      case "DEBUG":
        next.debug += 1;
        break;
      default:
        break;
    }
  }
  next.error_rate = next.total ? (next.error + next.fatal) / next.total : 0;
  return next;
}

export function nextReconnectDelay(prevMs: number): number {
  if (prevMs <= 0) return 1000;
  return Math.min(15000, prevMs * 2);
}

export function defaultWsBase(): string {
  return process.env.NEXT_PUBLIC_QUERY_WS_URL || "ws://127.0.0.1:8082/api/v1/stream";
}

export function streamUrl(ticket: string, projectId: string, base = defaultWsBase()): string {
  const u = new URL(base);
  u.searchParams.set("ticket", ticket);
  u.searchParams.set("project_id", projectId);
  return u.toString();
}

export async function resolveWsBase(): Promise<string> {
  try {
    const res = await fetch("/api/runtime", { cache: "no-store" });
    if (res.ok) {
      const data = (await res.json()) as { wsUrl?: string };
      if (typeof data.wsUrl === "string" && data.wsUrl.trim() !== "") {
        return data.wsUrl.trim();
      }
    }
  } catch {
    // Fall back to the compile-time default used by local `npm run dev`.
  }
  return defaultWsBase();
}

export function intervalMs(interval: string): number {
  switch (interval) {
    case "1m":
      return 60_000;
    case "5m":
      return 300_000;
    case "15m":
      return 900_000;
    case "1h":
      return 3_600_000;
    case "1d":
      return 86_400_000;
    default:
      return 60_000;
  }
}
