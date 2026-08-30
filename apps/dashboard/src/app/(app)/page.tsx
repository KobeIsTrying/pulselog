"use client";

import { ErrorChart, VolumeChart, applyLiveSeries, mergeSeries, type SeriesPoint } from "@/components/charts";
import { LiveBar } from "@/components/live-bar";
import { useLive } from "@/components/live-provider";
import { RefreshBar } from "@/components/refresh-bar";
import { useApp } from "@/components/providers";
import { Card, EmptyState, ErrorBanner, LevelBadge, Skeleton } from "@/components/ui";
import { api, statsQuery } from "@/lib/api";
import { ApiError } from "@/lib/errors";
import { bumpOverview, intervalMs } from "@/lib/live";
import { formatPct, rangeWindow } from "@/lib/time";
import type { ErrorGroup, OverviewStats, ServiceStat } from "@/lib/types";
import { usePoll } from "@/hooks/use-poll";
import { useCallback, useEffect, useRef, useState } from "react";

export default function OverviewPage() {
  const { project, range, pollMs } = useApp();
  const [overview, setOverview] = useState<OverviewStats | null>(null);
  const [series, setSeries] = useState<SeriesPoint[]>([]);
  const [services, setServices] = useState<ServiceStat[]>([]);
  const [errors, setErrors] = useState<ErrorGroup[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const { subscribe } = useLive();
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const load = useCallback(async () => {
    if (!project) return;
    setError("");
    const { start, end, interval } = rangeWindow(range);
    const base = { start, end, projectId: project.id, interval };
    try {
      const [ov, total, errTs, warnTs, infoTs, debugTs, svcs, errs] = await Promise.all([
        api.overview(statsQuery({ start, end, projectId: project.id })),
        api.timeseries(statsQuery(base)),
        api.timeseries(statsQuery({ ...base, level: "ERROR" })),
        api.timeseries(statsQuery({ ...base, level: "WARN" })),
        api.timeseries(statsQuery({ ...base, level: "INFO" })),
        api.timeseries(statsQuery({ ...base, level: "DEBUG" })),
        api.serviceStats(statsQuery({ start, end, projectId: project.id, sort: "error_count" })),
        api.commonErrors(statsQuery({ start, end, projectId: project.id })),
      ]);
      setOverview(ov);
      setSeries(
        mergeSeries([
          { level: "total", points: total.points || [] },
          { level: "ERROR", points: errTs.points || [] },
          { level: "WARN", points: warnTs.points || [] },
          { level: "INFO", points: infoTs.points || [] },
          { level: "DEBUG", points: debugTs.points || [] },
        ]),
      );
      setServices(svcs.services || []);
      setErrors(errs.errors || []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load overview.");
    } finally {
      setLoading(false);
    }
  }, [project, range]);

  useEffect(() => {
    setLoading(true);
    void load();
  }, [load]);
  usePoll(pollMs, load);

  useEffect(() => {
    return subscribe((events) => {
      if (!events.length) return;
      setOverview((cur) => (cur ? bumpOverview(cur, events) : cur));
      const { start, interval } = rangeWindow(range);
      const startMs = new Date(start).getTime();
      const nowMs = Date.now();
      setSeries((cur) => applyLiveSeries(cur, events, { startMs, intervalMs: intervalMs(interval), nowMs }));
      if (refreshTimer.current) clearTimeout(refreshTimer.current);
      refreshTimer.current = setTimeout(() => {
        void load();
      }, 1500);
    });
  }, [subscribe, range, load]);

  const errCount = (overview?.error || 0) + (overview?.fatal || 0);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Overview</h1>
          <p className="text-sm text-muted">{project ? `${project.name} · last ${range}` : "Select a project"}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <LiveBar />
          <RefreshBar onRefresh={() => void load()} loading={loading} />
        </div>
      </div>
      {error ? <ErrorBanner message={error} /> : null}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label="Total logs" value={overview?.total} loading={loading} />
        <Stat label="Errors" value={errCount} loading={loading} accent="error" hint={overview ? `${formatPct(overview.error_rate)} of traffic` : undefined} />
        <Stat label="Warnings" value={overview?.warn} loading={loading} accent="warn" />
        <Stat label="Active services" value={overview?.active_services} loading={loading} />
      </div>
      <div className="grid gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2 p-4">
          <h2 className="mb-3 text-sm font-medium">Logs over time</h2>
          {loading ? <Skeleton className="h-64" /> : series.length === 0 ? (
            <EmptyState title="No time-series points" body="Send events through the ingestion API to populate this chart." />
          ) : (
            <VolumeChart data={series} />
          )}
        </Card>
        <Card className="p-4">
          <h2 className="mb-3 text-sm font-medium">Error trend</h2>
          {loading ? <Skeleton className="h-48" /> : <ErrorChart data={series} />}
          {overview ? (
            <p className="mt-3 text-xs text-muted">
              {errCount} errors · {formatPct(overview.error_rate)} error rate in this window
            </p>
          ) : null}
        </Card>
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <div className="border-b border-border px-4 py-3 text-sm font-medium">Service activity</div>
          {loading ? (
            <div className="space-y-2 p-4">
              <Skeleton className="h-8" />
              <Skeleton className="h-8" />
            </div>
          ) : services.length === 0 ? (
            <EmptyState title="No services yet" body="Create a service and API key, then ingest events." />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="text-xs uppercase text-muted">
                  <tr>
                    <th className="px-4 py-2 font-medium">Service</th>
                    <th className="px-4 py-2 font-medium">Logs</th>
                    <th className="px-4 py-2 font-medium">Errors</th>
                    <th className="px-4 py-2 font-medium">Warnings</th>
                    <th className="px-4 py-2 font-medium">Error rate</th>
                  </tr>
                </thead>
                <tbody>
                  {services.map((s) => (
                    <tr key={s.service} className="border-t border-border">
                      <td className="px-4 py-2 font-mono text-xs">{s.service}</td>
                      <td className="tabular px-4 py-2">{s.total}</td>
                      <td className={`tabular px-4 py-2 ${s.error_count > 0 ? "text-error" : ""}`}>{s.error_count}</td>
                      <td className="tabular px-4 py-2">{s.warn_count}</td>
                      <td className="tabular px-4 py-2">{formatPct(s.error_rate)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
        <Card>
          <div className="border-b border-border px-4 py-3 text-sm font-medium">Frequent errors</div>
          {loading ? (
            <div className="p-4">
              <Skeleton className="h-24" />
            </div>
          ) : errors.length === 0 ? (
            <EmptyState title="No errors in this window" body="ERROR-level events will appear here by exact message." />
          ) : (
            <ul className="divide-y divide-border">
              {errors.map((e) => (
                <li key={e.message} className="flex items-start justify-between gap-3 px-4 py-3">
                  <div>
                    <LevelBadge level="ERROR" />
                    <p className="mt-1 text-sm">{e.message}</p>
                  </div>
                  <span className="tabular text-xs text-muted">{e.count}×</span>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  );
}

function Stat({ label, value, loading, accent, hint }: { label: string; value?: number; loading: boolean; accent?: string; hint?: string }) {
  return (
    <Card className="p-4">
      <p className="text-[11px] font-semibold uppercase tracking-wider text-muted">{label}</p>
      {loading ? <Skeleton className="mt-2 h-8 w-16" /> : <p className={`mt-1 text-2xl font-semibold tabular ${accent === "error" ? "text-error" : accent === "warn" ? "text-warn" : ""}`}>{value ?? 0}</p>}
      {hint ? <p className="mt-1 text-xs text-muted">{hint}</p> : null}
    </Card>
  );
}
