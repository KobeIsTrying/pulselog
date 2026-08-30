import type { LogEvent, OverviewStats, SessionUser } from "@/lib/types";

export const sessionUser: SessionUser = {
  user_id: "11111111-1111-4111-8111-111111111111",
  email: "owner@example.com",
  orgs: [{ org: { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", name: "Acme", slug: "acme" }, role: "owner" }],
  project_ids: ["bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"],
};

export const overview: OverviewStats = {
  total: 120,
  debug: 10,
  info: 80,
  warn: 20,
  error: 9,
  fatal: 1,
  error_rate: 0.083,
  active_services: 3,
};

export const sampleLog: LogEvent = {
  event_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
  project_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
  timestamp: "2026-08-29T12:00:00.000Z",
  ingested_at: "2026-08-29T12:00:01.000Z",
  service: "payment-service",
  level: "ERROR",
  message: "Payment authorization failed",
  metadata: { requestId: "req-1" },
};
