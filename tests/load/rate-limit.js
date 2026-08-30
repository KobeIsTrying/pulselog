import http from "k6/http";
import { check } from "k6";
import { Counter } from "k6/metrics";
import { ingestURL, jsonHeaders, makeEvent } from "./lib.js";

const limited = new Counter("pulselog_rate_limited_seen");

export const options = {
  vus: Number(__ENV.VUS || 8),
  duration: __ENV.DURATION || "8s",
};

export default function () {
  const payload = makeEvent(__ITER);
  const res = http.post(`${ingestURL()}/v1/logs`, JSON.stringify(payload.event), {
    headers: jsonHeaders({ "X-API-Key": payload.apiKey }),
    tags: { name: "ingest" },
  });
  check(res, {
    "202 or 429": (r) => r.status === 202 || r.status === 429,
  });
  if (res.status === 429) limited.add(1);
}
