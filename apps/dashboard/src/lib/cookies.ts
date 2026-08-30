export function cookieSecure(): boolean {
  const raw = (process.env.COOKIE_SECURE || "").trim().toLowerCase();
  if (raw === "true" || raw === "1" || raw === "yes" || raw === "on") {
    return true;
  }
  if (raw === "false" || raw === "0" || raw === "no" || raw === "off") {
    return false;
  }
  return process.env.NODE_ENV === "production";
}

export function tokenCookieOptions(maxAge = 60 * 60 * 24) {
  return {
    httpOnly: true as const,
    sameSite: "lax" as const,
    secure: cookieSecure(),
    path: "/",
    maxAge,
  };
}
