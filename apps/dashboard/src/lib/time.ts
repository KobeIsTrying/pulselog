import { TIME_RANGES, type TimeRangeKey } from "./types";

export function rangeWindow(key: TimeRangeKey, now = new Date()) {
  const spec = TIME_RANGES.find((r) => r.key === key) ?? TIME_RANGES[2];
  const end = now;
  const start = new Date(end.getTime() - spec.hours * 3600 * 1000);
  return {
    start: start.toISOString(),
    end: end.toISOString(),
    interval: spec.interval,
  };
}

export function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toISOString().replace("T", " ").replace("Z", " UTC");
}

export function formatShortTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

export function formatPct(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`;
}
