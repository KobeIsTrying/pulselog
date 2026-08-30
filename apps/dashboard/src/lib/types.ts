export type Role = "owner" | "admin" | "member" | "viewer";

export type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR" | "FATAL";

export interface FieldError {
  field: string;
  message: string;
}

export interface ApiErrorBody {
  error: string;
  message?: string;
  fields?: FieldError[];
}

export interface SessionUser {
  user_id: string;
  email: string;
  orgs: OrgMembership[];
  project_ids: string[];
}

export interface Org {
  id: string;
  name: string;
  slug: string;
}

export interface OrgMembership {
  org: Org;
  role: Role;
}

export interface Project {
  id: string;
  org_id: string;
  name: string;
  slug: string;
}

export interface Service {
  id: string;
  project_id: string;
  name: string;
}

export interface APIKey {
  id: string;
  project_id: string;
  service_id: string;
  service: string;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at?: string | null;
  revoked_at?: string | null;
}

export interface CreatedAPIKey {
  id: string;
  prefix: string;
  name: string;
  service: string;
  project_id: string;
  token: string;
}

export interface LogEvent {
  event_id: string;
  project_id?: string;
  timestamp: string;
  ingested_at: string;
  service: string;
  level: LogLevel | string;
  message: string;
  host?: string;
  trace_id?: string;
  metadata?: Record<string, string>;
}

export interface LogListResponse {
  logs: LogEvent[];
  page_size: number;
  has_more: boolean;
  next_cursor?: string;
}

export interface OverviewStats {
  total: number;
  debug: number;
  info: number;
  warn: number;
  error: number;
  fatal: number;
  error_rate: number;
  active_services: number;
}

export interface TimeBucket {
  bucket: string;
  count: number;
  service?: string;
  level?: string;
}

export interface TimeseriesResponse {
  interval: string;
  points: TimeBucket[];
}

export interface ServiceStat {
  service: string;
  total: number;
  error_count: number;
  warn_count: number;
  error_rate: number;
}

export interface ErrorGroup {
  message: string;
  count: number;
}

export type TimeRangeKey = "1h" | "6h" | "24h" | "7d";

export const TIME_RANGES: { key: TimeRangeKey; label: string; hours: number; interval: string }[] = [
  { key: "1h", label: "1h", hours: 1, interval: "1m" },
  { key: "6h", label: "6h", hours: 6, interval: "5m" },
  { key: "24h", label: "24h", hours: 24, interval: "15m" },
  { key: "7d", label: "7d", hours: 168, interval: "1h" },
];

export function canManageKeys(role: Role | undefined): boolean {
  return role === "owner" || role === "admin";
}

export function canManageProjects(role: Role | undefined): boolean {
  return role === "owner";
}

export function canManageServices(role: Role | undefined): boolean {
  return role === "owner" || role === "admin";
}
