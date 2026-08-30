const PUBLIC_PATHS = ["/login", "/signup"];

/** Returns a path to redirect to, or null if the request may continue. */
export function authRedirectPath(pathname: string, hasToken: boolean): "/login" | "/" | null {
  const isPublic = PUBLIC_PATHS.some((p) => pathname === p);
  if (!hasToken && !isPublic) return "/login";
  if (hasToken && isPublic) return "/";
  return null;
}
