import { NextResponse } from "next/server";

export function GET() {
  const wsUrl =
    process.env.QUERY_WS_PUBLIC_URL ||
    process.env.NEXT_PUBLIC_QUERY_WS_URL ||
    "ws://127.0.0.1:8082/api/v1/stream";
  return NextResponse.json({ wsUrl });
}
