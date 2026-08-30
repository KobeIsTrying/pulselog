"use client";

import { copyText } from "@/lib/copy";
import { formatTime } from "@/lib/time";
import type { LogEvent } from "@/lib/types";
import { useEffect, useState } from "react";
import { Button, LevelBadge } from "./ui";

export function LogDetail({ event, onClose }: { event: LogEvent; onClose: () => void }) {
  const [copied, setCopied] = useState("");
  async function copy(label: string, value: string) {
    const ok = await copyText(value);
    setCopied(ok ? label : "");
    setTimeout(() => setCopied(""), 1500);
  }

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const meta = event.metadata && Object.keys(event.metadata).length > 0 ? JSON.stringify(event.metadata, null, 2) : null;

  return (
    <div className="fixed inset-0 z-40 flex justify-end bg-black/50" role="dialog" aria-modal="true" aria-labelledby="log-detail-title">
      <button type="button" className="h-full flex-1 cursor-default" aria-label="Close event detail" onClick={onClose} />
      <aside className="flex h-full w-full max-w-lg flex-col border-l border-border bg-raised shadow-2xl">
        <div className="flex items-start justify-between gap-3 border-b border-border px-5 py-4">
          <div>
            <h2 id="log-detail-title" className="text-sm font-semibold">
              Event detail
            </h2>
            <p className="mt-1 font-mono text-xs text-muted">{event.event_id}</p>
          </div>
          <Button variant="ghost" onClick={onClose}>
            Close
          </Button>
        </div>
        <div className="flex-1 space-y-4 overflow-y-auto px-5 py-4 text-sm">
          <Field label="Event ID">
            <div className="flex items-center gap-2">
              <code className="break-all text-xs">{event.event_id}</code>
              <Button variant="outline" onClick={() => void copy("event", event.event_id)}>
                {copied === "event" ? "Copied" : "Copy"}
              </Button>
            </div>
          </Field>
          <Field label="Timestamp">{formatTime(event.timestamp)}</Field>
          {event.ingested_at ? <Field label="Ingested">{formatTime(event.ingested_at)}</Field> : null}
          <Field label="Level">
            <LevelBadge level={event.level} />
          </Field>
          <Field label="Service">
            <span className="font-mono text-xs">{event.service}</span>
          </Field>
          {event.project_id ? (
            <Field label="Project ID">
              <code className="text-xs">{event.project_id}</code>
            </Field>
          ) : null}
          {event.host ? <Field label="Host">{event.host}</Field> : null}
          {event.trace_id ? <Field label="Trace ID">{event.trace_id}</Field> : null}
          <Field label="Message">
            <p className="whitespace-pre-wrap">{event.message}</p>
          </Field>
          <Field label="Metadata">
            {meta ? (
              <pre className="overflow-x-auto rounded-md border border-border bg-bg p-3 font-mono text-xs">{meta}</pre>
            ) : (
              <p className="text-muted">No structured metadata on this event.</p>
            )}
          </Field>
        </div>
      </aside>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="text-[11px] font-semibold uppercase tracking-wider text-muted">{label}</p>
      <div className="mt-1">{children}</div>
    </div>
  );
}
