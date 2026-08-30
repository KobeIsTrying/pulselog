"use client";

import type { TimeBucket } from "@/lib/types";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

export interface SeriesPoint {
  t: string;
  label: string;
  ERROR?: number;
  WARN?: number;
  INFO?: number;
  DEBUG?: number;
  total?: number;
}

export function applyLiveSeries(
  series: SeriesPoint[],
  events: { timestamp: string; level: string }[],
  opts: { startMs: number; intervalMs: number; nowMs: number },
): SeriesPoint[] {
  const map = new Map(series.map((p) => [p.t, { ...p }]));
  for (const ev of events) {
    const t = new Date(ev.timestamp).getTime();
    if (Number.isNaN(t) || t < opts.startMs || t > opts.nowMs) continue;
    const bucket = Math.floor(t / opts.intervalMs) * opts.intervalMs;
    const key = new Date(bucket).toISOString();
    const cur = map.get(key) || { t: key, label: formatTick(key), total: 0 };
    cur.total = (cur.total || 0) + 1;
    const lv = ev.level.toUpperCase();
    if (lv === "ERROR" || lv === "FATAL") cur.ERROR = (cur.ERROR || 0) + 1;
    else if (lv === "WARN") cur.WARN = (cur.WARN || 0) + 1;
    else if (lv === "INFO") cur.INFO = (cur.INFO || 0) + 1;
    else if (lv === "DEBUG") cur.DEBUG = (cur.DEBUG || 0) + 1;
    map.set(key, cur);
  }
  return [...map.values()]
    .filter((p) => {
      const ts = new Date(p.t).getTime();
      return !Number.isNaN(ts) && ts >= opts.startMs;
    })
    .sort((a, b) => a.t.localeCompare(b.t));
}

export function mergeSeries(series: { level: string; points: TimeBucket[] }[]): SeriesPoint[] {
  const map = new Map<string, SeriesPoint>();
  for (const s of series) {
    for (const p of s.points) {
      const key = p.bucket;
      const cur = map.get(key) || { t: key, label: formatTick(key) };
      const n = Number(p.count) || 0;
      if (s.level === "ERROR") cur.ERROR = n;
      if (s.level === "WARN") cur.WARN = n;
      if (s.level === "INFO") cur.INFO = n;
      if (s.level === "DEBUG") cur.DEBUG = n;
      if (s.level === "total") cur.total = n;
      map.set(key, cur);
    }
  }
  return [...map.values()].sort((a, b) => a.t.localeCompare(b.t));
}

function formatTick(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", hour12: false });
}

export function VolumeChart({ data }: { data: SeriesPoint[] }) {
  return (
    <div className="h-64 w-full" role="img" aria-label="Logs over time by level">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid stroke="#2a313c" strokeDasharray="3 3" />
          <XAxis dataKey="label" tick={{ fill: "#8b95a7", fontSize: 11 }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fill: "#8b95a7", fontSize: 11 }} axisLine={false} tickLine={false} allowDecimals={false} width={36} />
          <Tooltip
            contentStyle={{ background: "#12151b", border: "1px solid #2a313c", borderRadius: 8, fontSize: 12 }}
            labelStyle={{ color: "#e8edf4" }}
          />
          <Area type="monotone" dataKey="DEBUG" stackId="1" stroke="#8b95a7" fill="#8b95a733" />
          <Area type="monotone" dataKey="INFO" stackId="1" stroke="#59c2ff" fill="#59c2ff33" />
          <Area type="monotone" dataKey="WARN" stackId="1" stroke="#e6b450" fill="#e6b45033" />
          <Area type="monotone" dataKey="ERROR" stackId="1" stroke="#f07178" fill="#f0717866" />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

export function ErrorChart({ data }: { data: SeriesPoint[] }) {
  return (
    <div className="h-48 w-full" role="img" aria-label="Errors over time">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid stroke="#2a313c" strokeDasharray="3 3" />
          <XAxis dataKey="label" tick={{ fill: "#8b95a7", fontSize: 11 }} axisLine={false} tickLine={false} />
          <YAxis tick={{ fill: "#8b95a7", fontSize: 11 }} axisLine={false} tickLine={false} allowDecimals={false} width={36} />
          <Tooltip contentStyle={{ background: "#12151b", border: "1px solid #2a313c", borderRadius: 8, fontSize: 12 }} />
          <Area type="monotone" dataKey="ERROR" stroke="#f07178" fill="#f0717844" />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
