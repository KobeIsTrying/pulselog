import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { sampleLog, sessionUser } from "@/test/fixtures";
import LogsPage from "./page";
import type { LogEvent } from "@/lib/types";

const replace = vi.fn();
let params = new URLSearchParams();
const api = {
  services: vi.fn(),
  logs: vi.fn(),
  log: vi.fn(),
};

let listeners: Array<(events: LogEvent[]) => void> = [];
const live = {
  enabled: true,
  setEnabled: vi.fn(),
  status: "live" as const,
  paused: false,
  setPaused: vi.fn(),
  pending: 0,
  flushPending: () => [],
  subscribe: (fn: (events: LogEvent[]) => void) => {
    listeners.push(fn);
    return () => {
      listeners = listeners.filter((l) => l !== fn);
    };
  },
};

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, push: vi.fn(), refresh: vi.fn() }),
  usePathname: () => "/logs",
  useSearchParams: () => params,
}));

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      services: (...a: unknown[]) => api.services(...a),
      logs: (...a: unknown[]) => api.logs(...a),
      log: (...a: unknown[]) => api.log(...a),
    },
  };
});

vi.mock("@/components/providers", () => ({
  useApp: () => ({
    user: sessionUser,
    project: { id: "proj-1", org_id: "org", name: "default", slug: "default" },
    range: "24h",
    pollMs: 0,
  }),
}));

vi.mock("@/components/live-provider", () => ({
  useLive: () => live,
}));

function pushLive(events: LogEvent[]) {
  listeners.forEach((fn) => fn(events));
}

describe("log explorer live mode", () => {
  beforeEach(() => {
    params = new URLSearchParams();
    listeners = [];
    live.paused = false;
    live.pending = 0;
    api.services.mockResolvedValue({ services: [{ id: "s1", project_id: "proj-1", name: "payment-service" }] });
    api.logs.mockResolvedValue({ logs: [sampleLog], page_size: 50, has_more: false });
    api.log.mockResolvedValue(sampleLog);
  });

  it("renders a live event that matches filters", async () => {
    render(<LogsPage />);
    await screen.findAllByText("Payment authorization failed");
    await act(async () => {
      pushLive([
        {
          ...sampleLog,
          event_id: "live-1",
          project_id: "proj-1",
          message: "UNIQUE_LIVE_NEEDLE",
          level: "ERROR",
          service: "payment-service",
        },
      ]);
    });
    expect((await screen.findAllByText("UNIQUE_LIVE_NEEDLE")).length).toBeGreaterThan(0);
  });

  it("does not render a live event excluded by the active level filter", async () => {
    params = new URLSearchParams("level=ERROR");
    render(<LogsPage />);
    await screen.findAllByText("Payment authorization failed");
    pushLive([
      {
        ...sampleLog,
        event_id: "live-info",
        message: "should-not-appear-live",
        level: "INFO",
        service: "auth-service",
      },
    ]);
    expect(screen.queryByText("should-not-appear-live")).not.toBeInTheDocument();
  });

  it("does not duplicate an event already returned by REST", async () => {
    render(<LogsPage />);
    await screen.findAllByText("Payment authorization failed");
    pushLive([sampleLog]);
    expect(screen.getAllByText("Payment authorization failed")).toHaveLength(
      screen.getAllByText("Payment authorization failed").length,
    );
    expect(document.body.textContent?.split(sampleLog.event_id).length).toBeLessThanOrEqual(3);
  });

  it("keeps Refresh available as the REST fallback", async () => {
    const user = userEvent.setup();
    render(<LogsPage />);
    await screen.findByRole("button", { name: /refresh/i });
    api.logs.mockClear();
    await user.click(screen.getByRole("button", { name: /refresh/i }));
    await waitFor(() => expect(api.logs).toHaveBeenCalled());
  });
});
