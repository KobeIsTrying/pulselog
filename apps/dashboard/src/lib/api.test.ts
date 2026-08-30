import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./errors";
import { api, logsQuery, statsQuery } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("query builders", () => {
  it("sends filters to the Query API, not as a local-only query", () => {
    const qs = logsQuery({
      projectId: "proj-1",
      service: "payment-service",
      level: "ERROR",
      q: "authorization",
      page_size: 50,
    });
    expect(qs).toContain("project_id=proj-1");
    expect(qs).toContain("service=payment-service");
    expect(qs).toContain("level=ERROR");
    expect(qs).toContain("q=authorization");
    expect(qs).toContain("page_size=50");
  });

  it("includes a cursor for keyset pagination", () => {
    expect(logsQuery({ cursor: "abc" })).toContain("cursor=abc");
  });

  it("builds stats query params", () => {
    expect(statsQuery({ start: "a", end: "b", level: "ERROR", interval: "15m" })).toContain("level=ERROR");
  });
});

describe("api client", () => {
  it("throws on login failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        text: async () => JSON.stringify({ error: "unauthorized", message: "invalid credentials" }),
      }),
    );
    await expect(api.login("a@b.com", "wrong-password")).rejects.toBeInstanceOf(ApiError);
  });

  it("maps 403 from the BFF", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        text: async () => JSON.stringify({ error: "forbidden" }),
      }),
    );
    await expect(api.apiKeys("proj")).rejects.toMatchObject({ status: 403 });
  });

  it("maps 429 rate limits", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 429,
        text: async () => JSON.stringify({ error: "rate_limited" }),
      }),
    );
    await expect(api.overview("?start=a&end=b")).rejects.toMatchObject({
      status: 429,
      message: expect.stringMatching(/rate limit/i),
    });
  });

  it("redirects to login on 401 outside auth routes", async () => {
    const assign = vi.fn();
    vi.stubGlobal("location", { pathname: "/logs", assign });
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        text: async () => JSON.stringify({ error: "unauthorized" }),
      }),
    );
    await expect(api.logs("")).rejects.toBeInstanceOf(ApiError);
    expect(assign).toHaveBeenCalledWith("/login");
  });
});
