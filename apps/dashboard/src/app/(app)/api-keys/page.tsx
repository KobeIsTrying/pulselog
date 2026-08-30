"use client";

import { useApp } from "@/components/providers";
import { Button, Card, EmptyState, ErrorBanner, Input, Label, Skeleton } from "@/components/ui";
import { api } from "@/lib/api";
import { copyText } from "@/lib/copy";
import { ApiError } from "@/lib/errors";
import { formatTime } from "@/lib/time";
import { canManageKeys, type APIKey, type CreatedAPIKey, type Service } from "@/lib/types";
import { useCallback, useEffect, useState } from "react";

export default function ApiKeysPage() {
  const { project, role } = useApp();
  const allowed = canManageKeys(role);
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [name, setName] = useState("");
  const [service, setService] = useState("");
  const [created, setCreated] = useState<CreatedAPIKey | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    if (!project || !allowed) {
      setLoading(false);
      return;
    }
    setError("");
    try {
      const [keyRes, svcRes] = await Promise.all([api.apiKeys(project.id), api.services(project.id)]);
      setKeys(keyRes.keys || []);
      const svcs = svcRes.services || [];
      setServices(svcs);
      setService((cur) => cur || svcs[0]?.name || "");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load API keys.");
    } finally {
      setLoading(false);
    }
  }, [project, allowed]);

  useEffect(() => {
    setLoading(true);
    void load();
  }, [load]);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!project || !name.trim() || !service) return;
    setBusy(true);
    setError("");
    try {
      const key = await api.createApiKey(project.id, name.trim(), service);
      setCreated(key);
      setName("");
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the key.");
    } finally {
      setBusy(false);
    }
  }

  async function revoke(id: string) {
    if (!confirm("Revoke this API key? Ingest with this secret will fail immediately.")) return;
    setBusy(true);
    setError("");
    try {
      await api.revokeApiKey(id);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not revoke the key.");
    } finally {
      setBusy(false);
    }
  }

  if (!allowed) {
    return (
      <div className="space-y-4">
        <h1 className="text-xl font-semibold">API Keys</h1>
        <ErrorBanner message="You do not have permission to do that. API key management requires an owner or admin role." />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">API Keys</h1>
        <p className="text-sm text-muted">
          Keys are bound to a project and service. The raw secret is shown once. Existing keys cannot be retrieved.
        </p>
      </div>
      {error ? <ErrorBanner message={error} /> : null}
      {created ? (
        <Card className="border-accent/40 p-4">
          <p className="text-sm font-medium">New key created</p>
          <p className="mt-1 text-sm text-warn">This key will not be shown again. Copy it now and store it securely.</p>
          <code className="mt-3 block break-all rounded-md border border-border bg-bg px-3 py-2 font-mono text-xs">{created.token}</code>
          <div className="mt-3 flex gap-2">
            <Button
              variant="outline"
              onClick={async () => {
                const ok = await copyText(created.token);
                setCopied(ok);
              }}
            >
              {copied ? "Copied" : "Copy key"}
            </Button>
            <Button variant="ghost" onClick={() => setCreated(null)}>
              Dismiss
            </Button>
          </div>
        </Card>
      ) : null}
      <Card className="p-4">
        <form onSubmit={(e) => void create(e)} className="grid gap-3 md:grid-cols-3">
          <div>
            <Label htmlFor="key-name">Name</Label>
            <Input id="key-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="ci-ingest" />
          </div>
          <div>
            <Label htmlFor="key-service">Service</Label>
            <select
              id="key-service"
              className="w-full rounded-md border border-border bg-bg px-3 py-2 text-sm"
              value={service}
              onChange={(e) => setService(e.target.value)}
            >
              {services.map((s) => (
                <option key={s.id} value={s.name}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>
          <div className="flex items-end">
            <Button type="submit" disabled={busy || !name.trim() || !service}>
              {busy ? "Working…" : "Create key"}
            </Button>
          </div>
        </form>
        {services.length === 0 ? <p className="mt-3 text-sm text-muted">Create a service before issuing a key.</p> : null}
      </Card>
      <Card>
        <div className="border-b border-border px-4 py-3 text-sm font-medium">Issued keys</div>
        {loading ? (
          <div className="p-4">
            <Skeleton className="h-20" />
          </div>
        ) : keys.length === 0 ? (
          <EmptyState title="No API keys" body="Create a key, then send events to the ingestion API with the X-API-Key header." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-muted">
                <tr>
                  <th className="px-4 py-2 font-medium">Name</th>
                  <th className="px-4 py-2 font-medium">Prefix</th>
                  <th className="px-4 py-2 font-medium">Service</th>
                  <th className="px-4 py-2 font-medium">Created</th>
                  <th className="px-4 py-2 font-medium">Status</th>
                  <th className="px-4 py-2 font-medium" />
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id} className="border-t border-border">
                    <td className="px-4 py-2">{k.name}</td>
                    <td className="px-4 py-2 font-mono text-xs">{k.prefix}</td>
                    <td className="px-4 py-2 font-mono text-xs">{k.service}</td>
                    <td className="px-4 py-2 text-xs text-muted">{formatTime(k.created_at)}</td>
                    <td className="px-4 py-2 text-xs">{k.revoked_at ? "Revoked" : "Active"}</td>
                    <td className="px-4 py-2 text-right">
                      {!k.revoked_at ? (
                        <Button variant="danger" disabled={busy} onClick={() => void revoke(k.id)}>
                          Revoke
                        </Button>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
