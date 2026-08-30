import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect, useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LiveProvider, useLive } from "./live-provider";

const app = {
  project: { id: "proj-a" } as { id: string } | null,
  user: { user_id: "u" } as { user_id: string } | null,
};

const streamTicket = vi.fn(async () => ({ ticket: "ticket-1", expires_in: 45 }));

vi.mock("./providers", () => ({
  useApp: () => app,
}));

vi.mock("@/lib/api", () => ({
  api: {
    streamTicket: () => streamTicket(),
  },
}));

class FakeWS {
  static instances: FakeWS[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  url: string;
  readyState = 0;
  constructor(url: string) {
    this.url = url;
    FakeWS.instances.push(this);
    queueMicrotask(() => {
      this.readyState = 1;
      this.onopen?.();
    });
  }
  close() {
    this.readyState = 3;
    this.onclose?.();
  }
}

function Probe() {
  const live = useLive();
  return (
    <div>
      <span data-testid="status">{live.status}</span>
      <span data-testid="pending">{live.pending}</span>
      <button type="button" onClick={() => live.setEnabled(true)}>
        enable
      </button>
      <button type="button" onClick={() => live.setPaused(true)}>
        pause
      </button>
      <button type="button" onClick={() => live.setPaused(false)}>
        resume
      </button>
    </div>
  );
}

function Seen() {
  const live = useLive();
  const [seen, setSeen] = useState(0);
  useEffect(() => live.subscribe(() => setSeen((c) => c + 1)), [live]);
  return <span data-testid="seen">{seen}</span>;
}

describe("LiveProvider", () => {
  beforeEach(() => {
    FakeWS.instances = [];
    streamTicket.mockClear();
    streamTicket.mockResolvedValue({ ticket: "ticket-1", expires_in: 45 });
    app.project = { id: "proj-a" };
    app.user = { user_id: "u" };
    vi.stubGlobal("WebSocket", FakeWS);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ wsUrl: "ws://127.0.0.1:8082/api/v1/stream" }),
      }),
    );
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("connects with a ticket and reports live", async () => {
    const user = userEvent.setup();
    render(
      <LiveProvider>
        <Probe />
      </LiveProvider>,
    );
    await user.click(screen.getByRole("button", { name: "enable" }));
    await waitFor(() => expect(streamTicket).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByTestId("status")).toHaveTextContent("live"));
    expect(FakeWS.instances.at(-1)?.url).toContain("project_id=proj-a");
    expect(FakeWS.instances.at(-1)?.url).toContain("ticket=ticket-1");
  });

  it("reconnects with backoff after a drop", async () => {
    const user = userEvent.setup();
    render(
      <LiveProvider>
        <Probe />
      </LiveProvider>,
    );
    await user.click(screen.getByRole("button", { name: "enable" }));
    await waitFor(() => expect(FakeWS.instances.length).toBe(1));
    streamTicket.mockResolvedValue({ ticket: "ticket-2", expires_in: 45 });
    act(() => {
      FakeWS.instances[0].close();
    });
    expect(screen.getByTestId("status")).toHaveTextContent("reconnecting");
    await waitFor(() => expect(streamTicket).toHaveBeenCalledTimes(2), { timeout: 3000 });
  }, 8000);

  it("opens a new subscription when the project changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <LiveProvider>
        <Probe />
      </LiveProvider>,
    );
    await user.click(screen.getByRole("button", { name: "enable" }));
    await waitFor(() => expect(FakeWS.instances.length).toBe(1));
    app.project = { id: "proj-b" };
    rerender(
      <LiveProvider>
        <Probe />
      </LiveProvider>,
    );
    await waitFor(() => expect(FakeWS.instances.at(-1)?.url).toContain("project_id=proj-b"));
  });

  it("buffers events while paused and flushes on resume", async () => {
    const user = userEvent.setup();
    render(
      <LiveProvider>
        <Probe />
        <Seen />
      </LiveProvider>,
    );
    await user.click(screen.getByRole("button", { name: "enable" }));
    await waitFor(() => expect(FakeWS.instances.length).toBe(1));
    await user.click(screen.getByRole("button", { name: "pause" }));
    act(() => {
      FakeWS.instances[0].onmessage?.({
        data: JSON.stringify({
          v: 1,
          type: "log.created",
          data: {
            event_id: "p1",
            service: "s",
            level: "INFO",
            message: "paused",
            timestamp: "2026-08-30T00:00:00Z",
          },
        }),
      });
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 250));
    });
    await waitFor(() => expect(screen.getByTestId("pending")).toHaveTextContent("1"));
    expect(screen.getByTestId("seen")).toHaveTextContent("0");
    await user.click(screen.getByRole("button", { name: "resume" }));
    await waitFor(() => expect(screen.getByTestId("seen")).toHaveTextContent("1"));
  });
});
