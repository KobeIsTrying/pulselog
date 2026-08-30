import http from "k6/http";
import ws from "k6/ws";
import { check } from "k6";
import { authHeader, ingestURL, jsonHeaders, jwt, makeEvent, projectID, queryURL, wsURL } from "./lib.js";

const ingestRate = Number(__ENV.RATE || 100);
const duration = __ENV.DURATION || "30s";

export const options = {
  summaryTrendStats: ["min", "med", "avg", "p(90)", "p(95)", "p(99)", "max"],
  scenarios: {
    ingest: {
      executor: "constant-arrival-rate",
      rate: ingestRate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs: Number(__ENV.VUS || 20),
      maxVUs: Number(__ENV.MAX_VUS || 80),
      exec: "ingest",
    },
    query: {
      executor: "constant-vus",
      vus: Number(__ENV.QUERY_VUS || 5),
      duration,
      exec: "query",
    },
    stream: {
      executor: "constant-vus",
      vus: Number(__ENV.WS_CLIENTS || 3),
      duration,
      exec: "stream",
    },
  },
};

export function ingest() {
  const payload = makeEvent(__ITER);
  const res = http.post(`${ingestURL()}/v1/logs`, JSON.stringify(payload.event), {
    headers: jsonHeaders({ "X-API-Key": payload.apiKey }),
    tags: { name: "ingest" },
  });
  check(res, { "ingest 202": (r) => r.status === 202 });
}

export function query() {
  const pid = projectID();
  const res = http.get(`${queryURL()}/api/v1/logs?project_id=${pid}&page_size=50`, {
    headers: authHeader(),
    tags: { name: "logs_recent" },
  });
  check(res, { "query 200": (r) => r.status === 200 });
  http.get(`${queryURL()}/api/v1/stats/overview?project_id=${pid}`, {
    headers: authHeader(),
    tags: { name: "stats_overview" },
  });
}

export function stream() {
  const ticketRes = http.post(`${queryURL()}/api/v1/stream/ticket`, null, {
    headers: Object.assign({ Authorization: `Bearer ${jwt()}` }, jsonHeaders()),
  });
  check(ticketRes, { "ticket 200": (r) => r.status === 200 });
  let ticket = "";
  try {
    ticket = JSON.parse(ticketRes.body).ticket;
  } catch (e) {
    return;
  }
  if (!ticket) return;
  const hold = Number(__ENV.WS_HOLD_MS || 25000);
  const url = `${wsURL()}/api/v1/stream?ticket=${ticket}&project_id=${projectID()}`;
  ws.connect(url, {}, (socket) => {
    socket.on("message", () => {});
    socket.setTimeout(() => socket.close(), hold);
  });
}
