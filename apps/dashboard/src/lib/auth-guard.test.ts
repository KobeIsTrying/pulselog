import { describe, expect, it } from "vitest";
import { authRedirectPath } from "./auth-guard";

describe("protected routing", () => {
  it("sends anonymous visitors to login", () => {
    expect(authRedirectPath("/", false)).toBe("/login");
    expect(authRedirectPath("/logs", false)).toBe("/login");
  });

  it("allows login and signup without a token", () => {
    expect(authRedirectPath("/login", false)).toBeNull();
    expect(authRedirectPath("/signup", false)).toBeNull();
  });

  it("sends authenticated users away from auth pages", () => {
    expect(authRedirectPath("/login", true)).toBe("/");
    expect(authRedirectPath("/signup", true)).toBe("/");
  });

  it("allows authenticated access to the app", () => {
    expect(authRedirectPath("/", true)).toBeNull();
    expect(authRedirectPath("/api-keys", true)).toBeNull();
  });
});
