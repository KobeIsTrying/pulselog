export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

export function messageForStatus(status: number, fallback?: string): string {
  if (status === 401) return "Your session expired. Sign in again.";
  if (status === 403) return "You do not have permission to do that.";
  if (status === 429) return "Rate limit reached. Wait a moment and try again.";
  if (status === 504) return "The query timed out. Narrow the time range and retry.";
  if (status === 503) return "The query service is unavailable. Check that query-api and ClickHouse are running.";
  if (status >= 500) return "Something went wrong on the server.";
  return fallback || "Request failed.";
}
