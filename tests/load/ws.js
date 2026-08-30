import http from "k6/http";
import ws from "k6/ws";
import { check } from "k6";
import { Counter } from "k6/metrics";
import { authHeader, jwt, jsonHeaders, projectID, queryURL, wsURL } from "./lib.js";

const frames = new Counter("pulselog_ws_frames");
const hellos = new Counter("pulselog_ws_hellos");

export const options = {
  vus: Number(__ENV.WS_CLIENTS || 1),
  duration: __ENV.DURATION || "25s",
};

export default function () {
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

  const url = `${wsURL()}/api/v1/stream?ticket=${ticket}&project_id=${projectID()}`;
  const hold = Number(__ENV.WS_HOLD_MS || 20000);
  const res = ws.connect(url, {}, (socket) => {
    socket.on("open", () => {});
    socket.on("message", (data) => {
      if (String(data).indexOf("hello") !== -1) hellos.add(1);
      else frames.add(1);
    });
    socket.on("error", () => {});
    socket.setTimeout(() => socket.close(), hold);
  });
  check(res, { "ws connected": (r) => r && r.status === 101 });
}
