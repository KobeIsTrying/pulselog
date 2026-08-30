import http from "k6/http";
import { check } from "k6";
import { Counter } from "k6/metrics";
import { ingestURL, jsonHeaders, makeEvent } from "./lib.js";

const accepted = new Counter("pulselog_ingest_accepted");
const rejected = new Counter("pulselog_ingest_rejected");

const rate = Number(__ENV.RATE || 100);
const duration = __ENV.DURATION || "20s";
const vus = Number(__ENV.VUS || 20);
const maxVUs = Number(__ENV.MAX_VUS || 80);

export const options = {
  summaryTrendStats: ["min", "med", "avg", "p(90)", "p(95)", "p(99)", "max"],
  scenarios: {
    ingest: {
      executor: "constant-arrival-rate",
      rate,
      timeUnit: "1s",
      duration,
      preAllocatedVUs: vus,
      maxVUs,
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.05"],
  },
};

export default function () {
  const payload = makeEvent(__ITER);
  const res = http.post(`${ingestURL()}/v1/logs`, JSON.stringify(payload.event), {
    headers: jsonHeaders({ "X-API-Key": payload.apiKey }),
    tags: { name: "ingest" },
  });
  const ok = check(res, {
    "accepted 202": (r) => r.status === 202,
  });
  if (ok) accepted.add(1);
  else rejected.add(1);
}
