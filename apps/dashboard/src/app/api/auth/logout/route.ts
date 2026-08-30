import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { forwardJson, TOKEN_COOKIE, tokenCookieOptions } from "@/lib/server/query-api";

export async function POST() {
  await forwardJson("POST", "/api/v1/auth/logout").catch(() => null);
  const jar = await cookies();
  jar.set(TOKEN_COOKIE, "", { ...tokenCookieOptions(0), maxAge: 0 });
  return NextResponse.json({ status: "logged_out" });
}
