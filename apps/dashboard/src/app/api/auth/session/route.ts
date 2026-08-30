import { NextResponse } from "next/server";
import { forwardJson, getToken } from "@/lib/server/query-api";

export async function GET() {
  const token = await getToken();
  if (!token) {
    return NextResponse.json({ error: "unauthorized", message: "authentication required" }, { status: 401 });
  }
  const res = await forwardJson("GET", "/api/v1/auth/me", { token });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    return NextResponse.json(data, { status: res.status });
  }
  return NextResponse.json(data);
}
