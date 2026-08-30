"use client";

import { useApp } from "@/components/providers";
import { Button, Card, EmptyState, ErrorBanner, Input, Label, Skeleton } from "@/components/ui";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/errors";
import { canManageProjects } from "@/lib/types";
import { useState } from "react";

export default function ProjectsPage() {
  const { org, projects, project, setProjectId, role, refreshSession } = useApp();
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const canCreate = canManageProjects(role);

  async function create(e: React.FormEvent) {
    e.preventDefault();
    if (!org || !name.trim()) return;
    setCreating(true);
    setError("");
    try {
      const created = await api.createProject(org.org.id, name.trim());
      setName("");
      await refreshSession();
      setProjectId(created.id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the project.");
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Projects</h1>
        <p className="text-sm text-muted">
          Isolation is enforced by the Query API. Switching project reloads dashboard data for that tenant.
        </p>
      </div>
      {error ? <ErrorBanner message={error} /> : null}
      {canCreate && org ? (
        <Card className="p-4">
          <form onSubmit={(e) => void create(e)} className="flex flex-wrap items-end gap-3">
            <div className="min-w-56 flex-1">
              <Label htmlFor="project-name">New project</Label>
              <Input id="project-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="production" />
            </div>
            <Button type="submit" disabled={creating || !name.trim()}>
              {creating ? "Creating…" : "Create project"}
            </Button>
          </form>
        </Card>
      ) : (
        <p className="text-sm text-muted">Creating projects requires the owner role. Member management is available on the Query API, not this UI.</p>
      )}
      <Card>
        <div className="border-b border-border px-4 py-3 text-sm font-medium">Your projects</div>
        {!org ? (
          <div className="p-4">
            <Skeleton className="h-16" />
          </div>
        ) : projects.length === 0 ? (
          <EmptyState title="No projects" body="An owner can create a project for this organization." />
        ) : (
          <ul className="divide-y divide-border">
            {projects.map((p) => (
              <li key={p.id} className="flex items-center justify-between gap-3 px-4 py-3">
                <div>
                  <p className="text-sm font-medium">{p.name}</p>
                  <p className="font-mono text-[11px] text-muted">{p.slug}</p>
                </div>
                {project?.id === p.id ? (
                  <span className="text-xs text-accent">Current</span>
                ) : (
                  <Button variant="outline" onClick={() => setProjectId(p.id)}>
                    Switch
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
