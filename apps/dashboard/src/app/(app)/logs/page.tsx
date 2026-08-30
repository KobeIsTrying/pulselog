"use client";

import { LiveBar } from "@/components/live-bar";
import { LogDetail } from "@/components/log-detail";
import { LogTable } from "@/components/log-table";
import { useLive } from "@/components/live-provider";
import { useApp } from "@/components/providers";
import { RefreshBar } from "@/components/refresh-bar";
import { Button, Card, EmptyState, ErrorBanner, Input, Label, Skeleton } from "@/components/ui";
import { usePoll } from "@/hooks/use-poll";
import { api, logsQuery } from "@/lib/api";
import { ApiError } from "@/lib/errors";
import { matchesLiveFilter, mergeLiveLogs } from "@/lib/live";
import { rangeWindow } from "@/lib/time";
import type { LogEvent, Service } from "@/lib/types";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";

const LEVELS = ["", "ERROR", "WARN", "INFO", "DEBUG", "FATAL"] as const;

function LogsExplorer() {
  const { project, range, pollMs } = useApp();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const service = searchParams.get("service") || "";
  const level = searchParams.get("level") || "";
  const qParam = searchParams.get("q") || "";
  const eventId = searchParams.get("event_id") || "";

  const [qInput, setQInput] = useState(qParam);
  const [services, setServices] = useState<Service[]>([]);
  const [logs, setLogs] = useState<LogEvent[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [cursor, setCursor] = useState<string | undefined>();
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<LogEvent | null>(null);
  const debounce = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { subscribe } = useLive();

  useEffect(() => {
    setQInput(qParam);
  }, [qParam]);

  const setParams = useCallback(
    (patch: Record<string, string>) => {
      const next = new URLSearchParams(searchParams.toString());
      for (const [k, v] of Object.entries(patch)) {
        if (v) next.set(k, v);
        else next.delete(k);
      }
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [pathname, router, searchParams],
  );

  useEffect(() => {
    if (!project) return;
    void api.services(project.id).then((r) => setServices(r.services || [])).catch(() => setServices([]));
  }, [project]);

  const query = useMemo(() => {
    const { start, end } = rangeWindow(range);
    return logsQuery({
      projectId: project?.id,
      service: service || undefined,
      level: level || undefined,
      start,
      end,
      q: qParam || undefined,
      event_id: eventId || undefined,
      page_size: 50,
    });
  }, [project?.id, service, level, range, qParam, eventId]);

  const filter = useMemo(
    () => ({
      projectId: project?.id,
      service: service || undefined,
      level: level || undefined,
      q: qParam || undefined,
      eventId: eventId || undefined,
    }),
    [project?.id, service, level, qParam, eventId],
  );

  const load = useCallback(async () => {
    if (!project?.id) return;
    setError("");
    setLoading(true);
    try {
      const res = await api.logs(query);
      setLogs((cur) => mergeLiveLogs(res.logs || [], cur.filter((e) => matchesLiveFilter(e, filter))));
      setHasMore(res.has_more);
      setCursor(res.next_cursor);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load logs.");
      setLogs([]);
    } finally {
      setLoading(false);
    }
  }, [project?.id, query, filter]);

  useEffect(() => {
    void load();
  }, [load]);
  usePoll(pollMs, load);

  useEffect(() => {
    return subscribe((events) => {
      const matched = events.filter((e) => matchesLiveFilter(e, filter));
      if (!matched.length) return;
      setLogs((cur) => mergeLiveLogs(cur, matched));
    });
  }, [subscribe, filter]);

  async function loadMore() {
    if (!cursor || !project) return;
    setLoadingMore(true);
    try {
      const res = await api.logs(
        logsQuery({
          projectId: project.id,
          service: service || undefined,
          level: level || undefined,
          start: rangeWindow(range).start,
          end: rangeWindow(range).end,
          q: qParam || undefined,
          event_id: eventId || undefined,
          page_size: 50,
          cursor,
        }),
      );
      setLogs((cur) => {
        const seen = new Set(cur.map((e) => e.event_id));
        return [...cur, ...(res.logs || []).filter((e) => e.event_id && !seen.has(e.event_id))];
      });
      setHasMore(res.has_more);
      setCursor(res.next_cursor);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load more logs.");
    } finally {
      setLoadingMore(false);
    }
  }

  function onSearchChange(value: string) {
    setQInput(value);
    if (debounce.current) clearTimeout(debounce.current);
    debounce.current = setTimeout(() => setParams({ q: value }), 350);
  }

  const active = [service && `service=${service}`, level && `level=${level}`, qParam && `q=${qParam}`, eventId && `event=${eventId}`].filter(Boolean);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Logs</h1>
          <p className="text-sm text-muted">Substring search on message. Filters run on the Query API, not in the browser.</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <LiveBar />
          <RefreshBar onRefresh={() => void load()} loading={loading} />
        </div>
      </div>
      <Card className="p-4">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div className="xl:col-span-2">
            <Label htmlFor="log-search">Search message</Label>
            <Input
              id="log-search"
              placeholder="Substring match, not ranked full-text"
              value={qInput}
              onChange={(e) => onSearchChange(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="log-service">Service</Label>
            <select
              id="log-service"
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-sm"
              value={service}
              onChange={(e) => setParams({ service: e.target.value })}
            >
              <option value="">All services</option>
              {services.map((s) => (
                <option key={s.id} value={s.name}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <Label htmlFor="log-level">Level</Label>
            <select
              id="log-level"
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-sm"
              value={level}
              onChange={(e) => setParams({ level: e.target.value })}
            >
              {LEVELS.map((l) => (
                <option key={l || "all"} value={l}>
                  {l || "All levels"}
                </option>
              ))}
            </select>
          </div>
          <div>
            <Label htmlFor="log-event">Event ID</Label>
            <Input id="log-event" value={eventId} onChange={(e) => setParams({ event_id: e.target.value.trim() })} placeholder="UUID" />
          </div>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {active.length ? (
            <p className="text-xs text-muted">
              Active: <span className="text-ink">{active.join(" · ")}</span>
            </p>
          ) : (
            <p className="text-xs text-muted">No extra filters. Time window follows the range control.</p>
          )}
          <Button
            variant="ghost"
            onClick={() => {
              setQInput("");
              router.replace(pathname, { scroll: false });
            }}
          >
            Clear filters
          </Button>
        </div>
      </Card>
      {error ? <ErrorBanner message={error} /> : null}
      <Card>
        {loading ? (
          <div className="space-y-2 p-4">
            <Skeleton className="h-8" />
            <Skeleton className="h-8" />
            <Skeleton className="h-8" />
          </div>
        ) : logs.length === 0 ? (
          <EmptyState title="No logs yet" body="Create an API key and send your first event through the ingestion API." />
        ) : (
          <LogTable
            logs={logs}
            selectedId={selected?.event_id}
            onSelect={(row) => {
              setSelected(row);
              void api.log(row.event_id).then(setSelected).catch(() => undefined);
            }}
          />
        )}
        {hasMore && !loading ? (
          <div className="border-t border-border p-3">
            <Button variant="outline" disabled={loadingMore} onClick={() => void loadMore()}>
              {loadingMore ? "Loading…" : "Load more"}
            </Button>
          </div>
        ) : null}
      </Card>
      {selected ? <LogDetail event={selected} onClose={() => setSelected(null)} /> : null}
    </div>
  );
}

export default function LogsPage() {
  return (
    <Suspense fallback={<Skeleton className="h-64" />}>
      <LogsExplorer />
    </Suspense>
  );
}
