import { describe, expect, it } from "vitest";
import { messageForStatus } from "./errors";

describe("API error messages", () => {
  it("explains 401", () => {
    expect(messageForStatus(401)).toMatch(/session expired/i);
  });

  it("explains 403", () => {
    expect(messageForStatus(403)).toMatch(/permission/i);
  });

  it("explains 429", () => {
    expect(messageForStatus(429)).toMatch(/rate limit/i);
  });

  it("explains 503 without leaking internals", () => {
    expect(messageForStatus(503)).toMatch(/unavailable/i);
    expect(messageForStatus(503)).not.toMatch(/stack|panic|sql/i);
  });
});
