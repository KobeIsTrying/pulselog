import http from "k6/http";
import { check } from "k6";
import { Trend } from "k6/metrics";
import { authHeader, projectID, queryURL, runID } from "./lib.js";

const firstPage = new Trend("pulselog_query_first_page_ms");
const laterPage = new Trend("pulselog_query_later_page_ms");
const byName = {
  logs_recent: new Trend("q_logs_recent_ms"),
  logs_service: new Trend("q_logs_service_ms"),
  logs_level: new Trend("q_logs_level_ms"),
  logs_combined: new Trend("q_logs_combined_ms"),
  logs_search: new Trend("q_logs_search_ms"),
  stats_overview: new Trend("q_stats_overview_ms"),
  stats_timeseries: new Trend("q_stats_timeseries_ms"),
  stats_services: new Trend("q_stats_services_ms"),
  stats_errors: new Trend("q_stats_errors_ms"),
  logs_page_first: new Trend("q_logs_page_first_ms"),
  logs_page_later: new Trend("q_logs_page_later_ms"),
  log_by_id: new Trend("q_log_by_id_ms"),
};

export const options = {
  summaryTrendStats: ["min", "med", "avg", "p(90)", "p(95)", "p(99)", "max"],
  vus: Number(__ENV.VUS || 8),
  duration: __ENV.DURATION || "20s",
  thresholds: {
    http_req_failed: ["rate<0.05"],
  },
};

function get(path, name) {
  const res = http.get(`${queryURL()}${path}`, {
    headers: authHeader(),
    tags: { name },
  });
  check(res, { [`${name} 200`]: (r) => r.status === 200 });
  if (byName[name]) {
    byName[name].add(res.timings.duration);
  }
  return res;
}

export default function () {
  const pid = projectID();
  const end = new Date().toISOString();
  const start = new Date(Date.now() - 24 * 3600 * 1000).toISOString();
  const qs = `project_id=${pid}&start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`;

  get(`/api/v1/logs?${qs}&page_size=50`, "logs_recent");
  get(`/api/v1/logs?${qs}&service=payment-service&page_size=50`, "logs_service");
  get(`/api/v1/logs?${qs}&level=ERROR&page_size=50`, "logs_level");
  get(`/api/v1/logs?${qs}&service=payment-service&level=ERROR&page_size=50`, "logs_combined");
  get(`/api/v1/logs?${qs}&q=${encodeURIComponent("bench:" + runID())}&page_size=20`, "logs_search");
  get(`/api/v1/stats/overview?${qs}`, "stats_overview");
  get(`/api/v1/stats/timeseries?${qs}&interval=1m`, "stats_timeseries");
  get(`/api/v1/stats/services?${qs}&sort=error_count`, "stats_services");
  get(`/api/v1/stats/errors?${qs}`, "stats_errors");

  const first = get(`/api/v1/logs?${qs}&page_size=20`, "logs_page_first");
  firstPage.add(first.timings.duration);
  let cursor = "";
  try {
    cursor = JSON.parse(first.body).next_cursor || "";
  } catch (e) {
    cursor = "";
  }
  if (cursor) {
    const later = get(`/api/v1/logs?${qs}&page_size=20&cursor=${encodeURIComponent(cursor)}`, "logs_page_later");
    laterPage.add(later.timings.duration);
  }

  if (__ENV.EVENT_ID) {
    get(`/api/v1/logs/${__ENV.EVENT_ID}`, "log_by_id");
  }
}
