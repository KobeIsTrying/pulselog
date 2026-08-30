"use client";

import { formatShortTime } from "@/lib/time";
import type { LogEvent } from "@/lib/types";
import { LevelBadge } from "./ui";

export function LogTable({
  logs,
  selectedId,
  onSelect,
}: {
  logs: LogEvent[];
  selectedId?: string;
  onSelect: (event: LogEvent) => void;
}) {
  return (
    <>
      <div className="hidden md:block overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="sticky top-0 bg-raised text-xs uppercase tracking-wider text-muted">
            <tr>
              <th className="px-3 py-2 font-medium">Timestamp</th>
              <th className="px-3 py-2 font-medium">Level</th>
              <th className="px-3 py-2 font-medium">Service</th>
              <th className="px-3 py-2 font-medium">Message</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((row) => (
              <tr
                key={row.event_id}
                tabIndex={0}
                className={`cursor-pointer border-t border-border hover:bg-hover focus-visible:bg-hover ${selectedId === row.event_id ? "bg-hover" : ""}`}
                onClick={() => onSelect(row)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect(row);
                  }
                }}
              >
                <td className="whitespace-nowrap px-3 py-2 font-mono text-xs text-muted">{formatShortTime(row.timestamp)}</td>
                <td className="px-3 py-2">
                  <LevelBadge level={row.level} />
                </td>
                <td className="px-3 py-2 font-mono text-xs">{row.service}</td>
                <td className="max-w-xl truncate px-3 py-2">{row.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <ul className="divide-y divide-border md:hidden">
        {logs.map((row) => (
          <li key={row.event_id}>
            <button
              type="button"
              className="w-full px-3 py-3 text-left hover:bg-hover"
              onClick={() => onSelect(row)}
            >
              <div className="flex items-center justify-between gap-2">
                <LevelBadge level={row.level} />
                <span className="font-mono text-[11px] text-muted">{formatShortTime(row.timestamp)}</span>
              </div>
              <p className="mt-1 font-mono text-xs text-muted">{row.service}</p>
              <p className="mt-1 text-sm">{row.message}</p>
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}
