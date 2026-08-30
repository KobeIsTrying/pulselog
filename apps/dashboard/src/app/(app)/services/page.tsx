"use client";

import { LiveBar } from "@/components/live-bar";
import { useLive } from "@/components/live-provider";
import { useApp } from "@/components/providers";
import { RefreshBar } from "@/components/refresh-bar";
import { Button, Card, EmptyState, ErrorBanner, Input, Label, Skeleton } from "@/components/ui";
import { usePoll } from "@/hooks/use-poll";
import { api, statsQuery } from "@/lib/api";
import { ApiError } from "@/lib/errors";
import { formatPct, rangeWindow } from "@/lib/time";
import { canManageServices, type Service, type ServiceStat } from "@/lib/types";
import { useCallback, useEffect, useRef, useState } from "react";

export default function ServicesPage() {
  const { project, range, pollMs, role } = useApp();
  const [catalog, setCatalog] = useState<Service[]>([]);
  const [stats, setStats] = useState<ServiceStat[]>([]);
  const [sort, setSort] = useState("error_count");
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const canCreate = canManageServices(role);
  const { subscribe } = useLive();
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const load = useCallback(async () => {
    if (!project) return;
    setError("");
    const { start, end } = rangeWindow(range);
    try {
      const [list, stat] = await Promise.all([
        api.services(project.id),
        api.serviceStats(statsQuery({ start, end, projectId: project.id, sort })),
      ]);
      setCatalog(list.services || []);
      setStats(stat.services || []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load services.");
    } finally {
      setLoading(false);
    }
  }, [project, range, sort]);

  useEffect(() => {
    setLoading(true);
    void load();
  }, [load]);
  usePoll(pollMs, load);

  useEffect(() => {
    return subscribe(() => {
      if (refreshTimer.current) clearTimeout(refreshTimer.current);
      refreshTimer.current = setTimeout(() => {
        void load();
      }, 1500);
    });
  }, [subscribe, load]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!project || !name.trim()) return;
    setCreating(true);
    setError("");
    try {
      await api.createService(project.id, name.trim());
      setName("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the service.");
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">Services</h1>
          <p className="text-sm text-muted">Activity from the Query API. High error volume is highlighted; the API does not report uptime.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="sr-only" htmlFor="svc-sort">
            Sort
          </label>
          <select
            id="svc-sort"
            className="rounded-md border border-border bg-raised px-2 py-1 text-xs"
            value={sort}
            onChange={(e) => setSort(e.target.value)}
          >
            <option value="error_count">Sort: errors</option>
            <option value="total">Sort: volume</option>
            <option value="error_rate">Sort: error rate</option>
          </select>
          <LiveBar />
          <RefreshBar onRefresh={() => void load()} loading={loading} />
        </div>
      </div>
      {error ? <ErrorBanner message={error} /> : null}
      {canCreate ? (
        <Card className="p-4">
          <form onSubmit={(e) => void create(e)} className="flex flex-wrap items-end gap-3">
            <div className="min-w-56 flex-1">
              <Label htmlFor="svc-name">New service</Label>
              <Input id="svc-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="payment-service" />
            </div>
            <Button type="submit" disabled={creating || !name.trim()}>
              {creating ? "Creating…" : "Create service"}
            </Button>
          </form>
        </Card>
      ) : (
        <p className="text-sm text-muted">Creating services requires an owner or admin role.</p>
      )}
      <Card>
        <div className="border-b border-border px-4 py-3 text-sm font-medium">Registered services</div>
        {loading ? (
          <div className="p-4">
            <Skeleton className="h-16" />
          </div>
        ) : catalog.length === 0 ? (
          <EmptyState title="No services" body="Create a service, then issue an API key bound to it." />
        ) : (
          <ul className="divide-y divide-border px-4 py-2 text-sm">
            {catalog.map((s) => (
              <li key={s.id} className="py-2 font-mono text-xs">
                {s.name}
              </li>
            ))}
          </ul>
        )}
      </Card>
      <Card>
        <div className="border-b border-border px-4 py-3 text-sm font-medium">Activity in this window</div>
        {loading ? (
          <div className="p-4">
            <Skeleton className="h-24" />
          </div>
        ) : stats.length === 0 ? (
          <EmptyState title="No log activity" body="Ingest events for these services to populate counts." />
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
                {stats.map((s) => {
                  const hot = s.error_count > 0 && s.error_rate >= 0.05;
                  return (
                    <tr key={s.service} className={`border-t border-border ${hot ? "bg-error/5" : ""}`}>
                      <td className="px-4 py-2 font-mono text-xs">
                        {s.service}
                        {hot ? <span className="ml-2 text-[11px] text-error">High error volume</span> : null}
                      </td>
                      <td className="tabular px-4 py-2">{s.total}</td>
                      <td className={`tabular px-4 py-2 ${s.error_count > 0 ? "text-error" : ""}`}>{s.error_count}</td>
                      <td className="tabular px-4 py-2">{s.warn_count}</td>
                      <td className="tabular px-4 py-2">{formatPct(s.error_rate)}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
