import { describe, expect, it } from "vitest";
import { sampleLog } from "@/test/fixtures";
import {
  bumpOverview,
  envelopeToLog,
  matchesLiveFilter,
  mergeLiveLogs,
  nextReconnectDelay,
  parseLiveFrame,
  streamUrl,
} from "./live";

describe("live helpers", () => {
  it("matches explorer filters", () => {
    expect(matchesLiveFilter(sampleLog, { service: "payment-service", level: "ERROR" })).toBe(true);
    expect(matchesLiveFilter(sampleLog, { service: "auth-service" })).toBe(false);
    expect(matchesLiveFilter(sampleLog, { level: "INFO" })).toBe(false);
    expect(matchesLiveFilter(sampleLog, { q: "authorization" })).toBe(true);
    expect(matchesLiveFilter(sampleLog, { q: "inventory" })).toBe(false);
    expect(matchesLiveFilter(sampleLog, { eventId: sampleLog.event_id })).toBe(true);
    expect(matchesLiveFilter(sampleLog, { projectId: "other" })).toBe(false);
  });

  it("suppresses duplicate event_id and caps rows", () => {
    const second = { ...sampleLog, event_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", message: "newer" };
    const merged = mergeLiveLogs([sampleLog], [sampleLog, second], 2);
    expect(merged).toHaveLength(2);
    expect(merged[0].event_id).toBe(second.event_id);
    expect(mergeLiveLogs([sampleLog], [sampleLog])).toHaveLength(1);
  });

  it("parses log.created and ignores hello or garbage", () => {
    const ev = envelopeToLog({
      v: 1,
      type: "log.created",
      data: {
        event_id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee",
        service: "auth-service",
        level: "INFO",
        message: "ok",
        timestamp: "2026-08-30T00:00:00Z",
      },
    });
    expect(ev?.service).toBe("auth-service");
    expect(envelopeToLog({ type: "hello" })).toBeNull();
    expect(parseLiveFrame("not-json")).toBeNull();
  });

  it("bumps overview totals without inventing extra analytics", () => {
    const next = bumpOverview(
      { total: 10, debug: 0, info: 8, warn: 1, error: 1, fatal: 0, error_rate: 0.1, active_services: 2 },
      [
        { ...sampleLog, level: "ERROR" },
        { ...sampleLog, event_id: "x", level: "WARN" },
      ],
    );
    expect(next.total).toBe(12);
    expect(next.error).toBe(2);
    expect(next.warn).toBe(2);
    expect(next.active_services).toBe(2);
    expect(next.error_rate).toBeCloseTo(2 / 12);
  });

  it("puts the ticket on the runtime WebSocket URL, never as a JWT query", () => {
    const url = streamUrl("ticket-1", "proj-1", "wss://api.pulselog.example.com/api/v1/stream");
    expect(url).toContain("ticket=ticket-1");
    expect(url).toContain("project_id=proj-1");
    expect(url.startsWith("wss://api.pulselog.example.com/api/v1/stream")).toBe(true);
    expect(url).not.toContain("Bearer");
  });

  it("backs off reconnects to 15s", () => {
    expect(nextReconnectDelay(0)).toBe(1000);
    expect(nextReconnectDelay(1000)).toBe(2000);
    expect(nextReconnectDelay(8000)).toBe(15000);
    expect(nextReconnectDelay(15000)).toBe(15000);
  });
});
