import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { tokenCookieOptions } from "@/lib/cookies";
import { TOKEN_COOKIE } from "@/lib/session";

export { TOKEN_COOKIE, tokenCookieOptions };

export function queryApiBase(): string {
  return (process.env.QUERY_API_URL || "http://127.0.0.1:8082").replace(/\/$/, "");
}

export async function getToken(): Promise<string | undefined> {
  return (await cookies()).get(TOKEN_COOKIE)?.value;
}

export async function forwardJson(
  method: string,
  path: string,
  opts?: { body?: unknown; search?: string; token?: string },
): Promise<Response> {
  const token = opts?.token ?? (await getToken());
  const url = `${queryApiBase()}${path}${opts?.search || ""}`;
  const headers: Record<string, string> = { Accept: "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (opts?.body !== undefined) headers["Content-Type"] = "application/json";
  return fetch(url, {
    method,
    headers,
    body: opts?.body !== undefined ? JSON.stringify(opts.body) : undefined,
    cache: "no-store",
  });
}

export async function passthrough(res: Response): Promise<NextResponse> {
  const text = await res.text();
  return new NextResponse(text, {
    status: res.status,
    headers: { "Content-Type": res.headers.get("Content-Type") || "application/json" },
  });
}
