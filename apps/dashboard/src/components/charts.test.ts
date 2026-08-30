import { describe, expect, it } from "vitest";
import { applyLiveSeries, mergeSeries } from "./charts";

describe("mergeSeries", () => {
  it("merges real buckets and does not invent missing points", () => {
    const merged = mergeSeries([
      { level: "ERROR", points: [{ bucket: "2026-08-29T12:00:00Z", count: 3 }] },
      { level: "INFO", points: [{ bucket: "2026-08-29T12:00:00Z", count: 8 }] },
    ]);
    expect(merged).toHaveLength(1);
    expect(merged[0].ERROR).toBe(3);
    expect(merged[0].INFO).toBe(8);
    expect(merged[0].WARN).toBeUndefined();
  });

  it("applies live points inside a rolling window and drops old buckets", () => {
    const start = Date.parse("2026-08-30T11:00:00.000Z");
    const now = Date.parse("2026-08-30T12:00:00.000Z");
    const next = applyLiveSeries(
      [{ t: "2026-08-30T10:00:00.000Z", label: "10:00", total: 4, ERROR: 1 }],
      [
        { timestamp: "2026-08-30T11:59:30.000Z", level: "ERROR" },
        { timestamp: "2026-08-30T09:00:00.000Z", level: "INFO" },
      ],
      { startMs: start, intervalMs: 60_000, nowMs: now },
    );
    expect(next.every((p) => Date.parse(p.t) >= start)).toBe(true);
    const last = next[next.length - 1];
    expect(last.ERROR).toBe(1);
    expect(last.total).toBe(1);
  });
});
