import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { LiveStatus } from "@/lib/live";
import { LiveBar } from "./live-bar";

const live = {
  enabled: false,
  setEnabled: vi.fn(),
  status: "disconnected" as LiveStatus,
  paused: false,
  setPaused: vi.fn(),
  pending: 0,
  flushPending: () => [],
  subscribe: () => () => undefined,
};

vi.mock("./live-provider", () => ({
  useLive: () => live,
}));

describe("LiveBar", () => {
  it("toggles live mode and shows reconnecting state", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<LiveBar />);
    await user.click(screen.getByRole("button", { name: "LIVE" }));
    expect(live.setEnabled).toHaveBeenCalledWith(true);

    live.enabled = true;
    live.status = "reconnecting";
    rerender(<LiveBar />);
    expect(screen.getByTestId("live-status")).toHaveTextContent("Reconnecting");

    live.status = "live";
    live.paused = true;
    live.pending = 37;
    rerender(<LiveBar />);
    expect(screen.getByTestId("live-status")).toHaveTextContent("Live");
    expect(screen.getByTestId("live-pending")).toHaveTextContent("37 new logs");
    await user.click(screen.getByRole("button", { name: /resume live stream/i }));
    expect(live.setPaused).toHaveBeenCalledWith(false);
  });
});
