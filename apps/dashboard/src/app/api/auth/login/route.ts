import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { forwardJson, TOKEN_COOKIE, tokenCookieOptions } from "@/lib/server/query-api";

export async function POST(req: Request) {
  const body = await req.json().catch(() => null);
  if (!body || typeof body.email !== "string" || typeof body.password !== "string") {
    return NextResponse.json({ error: "invalid_request", message: "Email and password are required." }, { status: 400 });
  }
  const res = await forwardJson("POST", "/api/v1/auth/login", { body, token: "" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok || typeof data.token !== "string") {
    return NextResponse.json(
      { error: data.error || "unauthorized", message: data.message || "Invalid credentials." },
      { status: res.status || 401 },
    );
  }
  const jar = await cookies();
  jar.set(TOKEN_COOKIE, data.token, tokenCookieOptions());
  return NextResponse.json({
    user_id: data.user_id,
    email: data.email,
    expires_at: data.expires_at,
  });
}
