import { ApiError, messageForStatus } from "./errors";
import type {
  APIKey,
  CreatedAPIKey,
  ErrorGroup,
  LogEvent,
  LogListResponse,
  OverviewStats,
  Project,
  Service,
  ServiceStat,
  SessionUser,
  TimeseriesResponse,
} from "./types";

async function parse(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return {};
  try {
    return JSON.parse(text);
  } catch {
    return { message: text };
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
    cache: "no-store",
  });
  const data = (await parse(res)) as { error?: string; message?: string };
  if (!res.ok) {
    if (res.status === 401 && shouldBounceToLogin(path)) {
      window.location.assign("/login");
    }
    throw new ApiError(res.status, data.error || "error", messageForStatus(res.status, data.message));
  }
  return data as T;
}

function shouldBounceToLogin(path: string): boolean {
  if (typeof window === "undefined") return false;
  const here = window.location.pathname;
  if (here === "/login" || here === "/signup") return false;
  return !path.startsWith("/api/auth/");
}

function backend(path: string): string {
  return `/api/backend${path.startsWith("/") ? path : `/${path}`}`;
}

export const api = {
  login(email: string, password: string) {
    return request<{ user_id: string; email: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
  },
  register(email: string, password: string, organization: string) {
    return request<{ user_id: string; email: string; organization: { id: string }; project: { id: string } }>(
      "/api/auth/register",
      { method: "POST", body: JSON.stringify({ email, password, organization }) },
    );
  },
  logout() {
    return request<{ status: string }>("/api/auth/logout", { method: "POST" });
  },
  session() {
    return request<SessionUser>("/api/auth/session");
  },
  projects(orgId: string) {
    return request<{ projects: Project[] }>(backend(`/orgs/${orgId}/projects`));
  },
  createProject(orgId: string, name: string) {
    return request<Project>(backend(`/orgs/${orgId}/projects`), {
      method: "POST",
      body: JSON.stringify({ name }),
    });
  },
  services(projectId: string) {
    return request<{ services: Service[] }>(backend(`/projects/${projectId}/services`));
  },
  createService(projectId: string, name: string) {
    return request<Service>(backend(`/projects/${projectId}/services`), {
      method: "POST",
      body: JSON.stringify({ name }),
    });
  },
  apiKeys(projectId: string) {
    return request<{ keys: APIKey[] }>(backend(`/projects/${projectId}/api-keys`));
  },
  createApiKey(projectId: string, name: string, service: string) {
    return request<CreatedAPIKey>(backend(`/projects/${projectId}/api-keys`), {
      method: "POST",
      body: JSON.stringify({ name, service }),
    });
  },
  revokeApiKey(keyId: string) {
    return request<{ status: string }>(backend(`/api-keys/${keyId}`), { method: "DELETE" });
  },
  overview(qs: string) {
    return request<OverviewStats>(backend(`/stats/overview${qs}`));
  },
  timeseries(qs: string) {
    return request<TimeseriesResponse>(backend(`/stats/timeseries${qs}`));
  },
  serviceStats(qs: string) {
    return request<{ services: ServiceStat[] }>(backend(`/stats/services${qs}`));
  },
  commonErrors(qs: string) {
    return request<{ errors: ErrorGroup[] }>(backend(`/stats/errors${qs}`));
  },
  logs(qs: string) {
    return request<LogListResponse>(backend(`/logs${qs}`));
  },
  log(eventId: string) {
    return request<LogEvent>(backend(`/logs/${eventId}`));
  },
  streamTicket() {
    return request<{ ticket: string; expires_in: number }>("/api/auth/stream-ticket", { method: "POST" });
  },
};

export function statsQuery(params: { start: string; end: string; projectId?: string; interval?: string; level?: string; service?: string; sort?: string; q?: string }) {
  const u = new URLSearchParams();
  u.set("start", params.start);
  u.set("end", params.end);
  if (params.projectId) u.set("project_id", params.projectId);
  if (params.interval) u.set("interval", params.interval);
  if (params.level) u.set("level", params.level);
  if (params.service) u.set("service", params.service);
  if (params.sort) u.set("sort", params.sort);
  if (params.q) u.set("q", params.q);
  return `?${u.toString()}`;
}

export function logsQuery(params: {
  projectId?: string;
  service?: string;
  level?: string;
  start?: string;
  end?: string;
  q?: string;
  event_id?: string;
  page_size?: number;
  cursor?: string;
}) {
  const u = new URLSearchParams();
  if (params.projectId) u.set("project_id", params.projectId);
  if (params.service) u.set("service", params.service);
  if (params.level) u.set("level", params.level);
  if (params.start) u.set("start", params.start);
  if (params.end) u.set("end", params.end);
  if (params.q) u.set("q", params.q);
  if (params.event_id) u.set("event_id", params.event_id);
  if (params.page_size) u.set("page_size", String(params.page_size));
  if (params.cursor) u.set("cursor", params.cursor);
  const s = u.toString();
  return s ? `?${s}` : "";
}
