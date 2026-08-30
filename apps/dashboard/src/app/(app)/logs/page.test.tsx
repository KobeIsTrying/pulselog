import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { sampleLog, sessionUser } from "@/test/fixtures";
import LogsPage from "./page";

const replace = vi.fn();
let params = new URLSearchParams();
const api = {
  services: vi.fn(),
  logs: vi.fn(),
  log: vi.fn(),
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

describe("log explorer filters", () => {
  beforeEach(() => {
    params = new URLSearchParams();
    replace.mockReset();
    api.services.mockResolvedValue({ services: [{ id: "s1", project_id: "proj-1", name: "payment-service" }] });
    api.logs.mockResolvedValue({ logs: [sampleLog], page_size: 50, has_more: true, next_cursor: "cursor-1" });
    api.log.mockResolvedValue(sampleLog);
  });

  it("renders logs from the Query API", async () => {
    render(<LogsPage />);
    expect((await screen.findAllByText("Payment authorization failed")).length).toBeGreaterThan(0);
    expect(api.logs).toHaveBeenCalled();
    expect(String(api.logs.mock.calls[0]?.[0])).toContain("project_id=proj-1");
  });

  it("writes the service filter to the URL", async () => {
    const user = userEvent.setup();
    render(<LogsPage />);
    await screen.findByLabelText("Service");
    await user.selectOptions(screen.getByLabelText("Service"), "payment-service");
    expect(replace).toHaveBeenCalled();
    expect(String(replace.mock.calls.at(-1)?.[0])).toContain("service=payment-service");
  });

  it("writes the level filter to the URL", async () => {
    const user = userEvent.setup();
    render(<LogsPage />);
    await screen.findByLabelText("Level");
    await user.selectOptions(screen.getByLabelText("Level"), "ERROR");
    expect(String(replace.mock.calls.at(-1)?.[0])).toContain("level=ERROR");
  });

  it("debounces search into the Query API q parameter", async () => {
    const user = userEvent.setup();
    render(<LogsPage />);
    await screen.findByLabelText(/search message/i);
    await user.type(screen.getByLabelText(/search message/i), "auth");
    await waitFor(() => expect(String(replace.mock.calls.at(-1)?.[0])).toContain("q=auth"));
  });

  it("requests the next cursor page", async () => {
    const user = userEvent.setup();
    render(<LogsPage />);
    const more = await screen.findByRole("button", { name: /load more/i });
    api.logs.mockClear();
    await user.click(more);
    expect(String(api.logs.mock.calls[0]?.[0])).toContain("cursor=cursor-1");
  });
});
