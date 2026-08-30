import { NextResponse } from "next/server";
import { forwardJson, getToken, passthrough } from "@/lib/server/query-api";

async function handle(req: Request, path: string[]) {
  const token = await getToken();
  if (!token) {
    return NextResponse.json({ error: "unauthorized", message: "authentication required" }, { status: 401 });
  }
  const url = new URL(req.url);
  const target = `/api/v1/${path.join("/")}`;
  let body: unknown;
  if (req.method !== "GET" && req.method !== "HEAD") {
    const text = await req.text();
    body = text ? JSON.parse(text) : undefined;
  }
  const res = await forwardJson(req.method, target, {
    search: url.search,
    body,
    token,
  });
  return passthrough(res);
}

export async function GET(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  return handle(req, (await ctx.params).path);
}
export async function POST(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  return handle(req, (await ctx.params).path);
}
export async function PATCH(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  return handle(req, (await ctx.params).path);
}
export async function DELETE(req: Request, ctx: { params: Promise<{ path: string[] }> }) {
  return handle(req, (await ctx.params).path);
}
