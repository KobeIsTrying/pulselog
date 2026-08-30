"use client";

import { Button } from "@/components/ui";
import { useLive } from "./live-provider";

const statusLabel: Record<string, string> = {
  live: "Live",
  reconnecting: "Reconnecting",
  disconnected: "Disconnected",
};

export function LiveBar() {
  const { enabled, setEnabled, status, paused, setPaused, pending } = useLive();
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button
        variant={enabled ? "primary" : "outline"}
        aria-pressed={enabled}
        onClick={() => setEnabled(!enabled)}
      >
        LIVE
      </Button>
      {enabled ? (
        <>
          <span
            className={`inline-flex items-center gap-1.5 rounded-full border border-border px-2 py-0.5 text-[11px] font-medium ${
              status === "live" ? "text-info" : status === "reconnecting" ? "text-warn" : "text-muted"
            }`}
            data-testid="live-status"
          >
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                status === "live" ? "bg-info" : status === "reconnecting" ? "bg-warn" : "bg-muted"
              }`}
              aria-hidden
            />
            {statusLabel[status] || status}
          </span>
          <Button variant="outline" onClick={() => setPaused(!paused)}>
            {paused ? "Resume live stream" : "Pause live stream"}
          </Button>
          {paused && pending > 0 ? (
            <span className="text-xs text-muted" data-testid="live-pending">
              {pending} new logs
            </span>
          ) : null}
        </>
      ) : (
        <span className="text-xs text-muted">Polling and Refresh still work when live is off.</span>
      )}
    </div>
  );
}

export function LiveIndicator() {
  const { enabled, status } = useLive();
  if (!enabled) return null;
  return (
    <span className={`text-[11px] ${status === "live" ? "text-info" : status === "reconnecting" ? "text-warn" : "text-muted"}`}>
      {statusLabel[status] || status}
    </span>
  );
}