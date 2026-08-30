import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { forwardJson, TOKEN_COOKIE, tokenCookieOptions } from "@/lib/server/query-api";

export async function POST(req: Request) {
  const body = await req.json().catch(() => null);
  if (!body || typeof body.email !== "string" || typeof body.password !== "string") {
    return NextResponse.json({ error: "invalid_request", message: "Email, password, and organization are required." }, { status: 400 });
  }
  const res = await forwardJson("POST", "/api/v1/auth/register", {
    body: {
      email: body.email,
      password: body.password,
      organization: body.organization || "My organization",
    },
    token: "",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok || typeof data.token !== "string") {
    return NextResponse.json(
      { error: data.error || "invalid_request", message: data.message || "Could not create account." },
      { status: res.status || 400 },
    );
  }
  const jar = await cookies();
  jar.set(TOKEN_COOKIE, data.token, tokenCookieOptions());
  return NextResponse.json(
    {
      user_id: data.user_id,
      email: data.email,
      organization: data.organization,
      project: data.project,
    },
    { status: 201 },
  );
}
