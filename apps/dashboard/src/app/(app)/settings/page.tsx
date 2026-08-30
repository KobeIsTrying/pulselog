"use client";

import { useApp } from "@/components/providers";
import { Card } from "@/components/ui";

export default function SettingsPage() {
  const { user, org, project } = useApp();
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Settings</h1>
        <p className="text-sm text-muted">Account and tenancy context from the Query API.</p>
      </div>
      <Card className="p-5 space-y-3 text-sm">
        <Row label="Email" value={user?.email} />
        <Row label="User ID" value={user?.user_id} mono />
        <Row label="Organization" value={org?.org.name} />
        <Row label="Role" value={org?.role} />
        <Row label="Current project" value={project?.name} />
      </Card>
      <Card className="p-5 text-sm text-muted space-y-2">
        <p>Password reset, email verification, and MFA are not available in this phase.</p>
        <p>Live streaming is not enabled. Use Refresh or a 10s / 30s / 60s poll interval.</p>
        <p>Message search is a case-insensitive substring, not ranked full-text search.</p>
      </Card>
    </div>
  );
}

function Row({ label, value, mono }: { label: string; value?: string; mono?: boolean }) {
  return (
    <div className="flex flex-wrap justify-between gap-2 border-b border-border pb-3 last:border-0 last:pb-0">
      <span className="text-muted">{label}</span>
      <span className={mono ? "font-mono text-xs" : ""}>{value || "—"}</span>
    </div>
  );
}
