import { afterEach, describe, expect, it, vi } from "vitest";
import { cookieSecure, tokenCookieOptions } from "./cookies";

describe("cookie options", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("uses Secure in production unless COOKIE_SECURE=false", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("COOKIE_SECURE", "");
    expect(cookieSecure()).toBe(true);
    vi.stubEnv("COOKIE_SECURE", "false");
    expect(cookieSecure()).toBe(false);
  });

  it("keeps cookies insecure on local HTTP unless explicitly enabled", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("COOKIE_SECURE", "");
    expect(cookieSecure()).toBe(false);
    const opts = tokenCookieOptions();
    expect(opts.httpOnly).toBe(true);
    expect(opts.sameSite).toBe("lax");
    expect(opts.secure).toBe(false);
  });
});
