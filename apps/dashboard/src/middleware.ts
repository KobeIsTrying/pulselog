import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { authRedirectPath } from "@/lib/auth-guard";
import { TOKEN_COOKIE } from "@/lib/session";

export function middleware(req: NextRequest) {
  const { pathname } = req.nextUrl;
  if (pathname.startsWith("/api/")) return NextResponse.next();
  const token = req.cookies.get(TOKEN_COOKIE)?.value;
  const dest = authRedirectPath(pathname, Boolean(token));
  if (dest === "/login") {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    url.searchParams.set("next", pathname);
    return NextResponse.redirect(url);
  }
  if (dest === "/") {
    const url = req.nextUrl.clone();
    url.pathname = "/";
    url.search = "";
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
