import { NextResponse } from "next/server";
import { forwardJson, getToken, passthrough } from "@/lib/server/query-api";

export async function POST() {
  const token = await getToken();
  if (!token) {
    return NextResponse.json({ error: "unauthorized", message: "authentication required" }, { status: 401 });
  }
  const res = await forwardJson("POST", "/api/v1/stream/ticket", { token });
  return passthrough(res);
}
