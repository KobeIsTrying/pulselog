export const services = [
  "payment-service",
  "auth-service",
  "inventory-service",
  "notification-service",
  "order-service",
];

export const levels = ["DEBUG", "INFO", "INFO", "INFO", "WARN", "ERROR"];

export function uuid() {
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export function ingestURL() {
  return __ENV.INGEST_URL || "http://127.0.0.1:8080";
}

export function queryURL() {
  return __ENV.QUERY_URL || "http://127.0.0.1:8082";
}

export function wsURL() {
  return __ENV.QUERY_WS_URL || "ws://127.0.0.1:8082";
}

export function runID() {
  return __ENV.RUN_ID || "local";
}

export function apiKey() {
  const k = __ENV.PULSELOG_API_KEY;
  if (!k) {
    throw new Error("PULSELOG_API_KEY is required");
  }
  return k;
}

export function keysByService() {
  let mapped = {};
  if (__ENV.PULSELOG_API_KEYS) {
    mapped = JSON.parse(__ENV.PULSELOG_API_KEYS);
  }
  const fallback = __ENV.PULSELOG_API_KEY;
  const out = {};
  for (let i = 0; i < services.length; i++) {
    const name = services[i];
    out[name] = mapped[name] || fallback;
  }
  return out;
}

export function jwt() {
  const t = __ENV.PULSELOG_JWT;
  if (!t) {
    throw new Error("PULSELOG_JWT is required");
  }
  return t;
}

export function projectID() {
  const id = __ENV.PULSELOG_PROJECT_ID;
  if (!id) {
    throw new Error("PULSELOG_PROJECT_ID is required");
  }
  return id;
}

export function makeEvent(seq) {
  const service = pick(services);
  const level = pick(levels);
  const id = uuid();
  return {
    apiKey: keysByService()[service] || apiKey(),
    event: {
      event_id: id,
      service,
      level,
      message: `bench:${runID()} ${service} ${level} seq=${seq || 0} ${id}`,
      timestamp: new Date().toISOString(),
      host: `bench-host-${__VU || 0}`,
      metadata: {
        run_id: runID(),
        seq: String(seq || 0),
        source: "k6",
      },
    },
  };
}

export function authHeader() {
  return { Authorization: `Bearer ${jwt()}` };
}

export function jsonHeaders(extra) {
  return Object.assign({ Accept: "application/json", "Content-Type": "application/json" }, extra || {});
}
