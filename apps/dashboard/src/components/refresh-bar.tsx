"use client";

import { TIME_RANGES, type TimeRangeKey } from "@/lib/types";
import { Button } from "./ui";
import { useApp } from "./providers";
import { RefreshCw } from "lucide-react";

export function RefreshBar({ onRefresh, loading }: { onRefresh: () => void; loading?: boolean }) {
  const { range, setRange, pollMs, setPollMs } = useApp();
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="inline-flex rounded-md border border-border" role="group" aria-label="Time range">
        {TIME_RANGES.map((r) => (
          <button
            key={r.key}
            type="button"
            onClick={() => setRange(r.key as TimeRangeKey)}
            className={`px-2.5 py-1 text-xs ${range === r.key ? "bg-hover text-ink" : "text-muted hover:text-ink"}`}
            aria-pressed={range === r.key}
          >
            {r.label}
          </button>
        ))}
      </div>
      <label className="sr-only" htmlFor="poll">
        Auto refresh
      </label>
      <select
        id="poll"
        className="rounded-md border border-border bg-raised px-2 py-1 text-xs"
        value={String(pollMs)}
        onChange={(e) => setPollMs(Number(e.target.value) as 0 | 10000 | 30000 | 60000)}
      >
        <option value="0">Refresh: Off</option>
        <option value="10000">Every 10s</option>
        <option value="30000">Every 30s</option>
        <option value="60000">Every 60s</option>
      </select>
      <Button variant="outline" onClick={onRefresh} disabled={loading} aria-label="Refresh data">
        <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} aria-hidden />
        Refresh
      </Button>
    </div>
  );
}
