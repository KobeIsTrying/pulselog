import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { overview, sessionUser } from "@/test/fixtures";
import OverviewPage from "./page";

const api = {
  overview: vi.fn(),
  timeseries: vi.fn(),
  serviceStats: vi.fn(),
  commonErrors: vi.fn(),
};

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      overview: (...a: unknown[]) => api.overview(...a),
      timeseries: (...a: unknown[]) => api.timeseries(...a),
      serviceStats: (...a: unknown[]) => api.serviceStats(...a),
      commonErrors: (...a: unknown[]) => api.commonErrors(...a),
    },
  };
});

vi.mock("@/components/providers", () => ({
  useApp: () => ({
    user: sessionUser,
    project: { id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", org_id: "org", name: "default", slug: "default" },
    range: "24h",
    pollMs: 0,
  }),
}));

vi.mock("@/components/charts", () => ({
  mergeSeries: () => [],
  VolumeChart: () => <div>volume-chart</div>,
  ErrorChart: () => <div>error-chart</div>,
}));

describe("overview", () => {
  beforeEach(() => {
    api.overview.mockResolvedValue(overview);
    api.timeseries.mockResolvedValue({ interval: "15m", points: [] });
    api.serviceStats.mockResolvedValue({
      services: [{ service: "payment-service", total: 40, error_count: 4, warn_count: 2, error_rate: 0.1 }],
    });
    api.commonErrors.mockResolvedValue({ errors: [] });
  });

  it("renders real overview statistics", async () => {
    render(<OverviewPage />);
    expect(await screen.findByText("120")).toBeInTheDocument();
    expect(screen.getByText("Total logs")).toBeInTheDocument();
    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("payment-service")).toBeInTheDocument();
  });

  it("shows an empty state when there are no errors", async () => {
    render(<OverviewPage />);
    expect(await screen.findByText(/no errors in this window/i)).toBeInTheDocument();
  });

  it("shows an API failure message", async () => {
    const { ApiError } = await import("@/lib/errors");
    api.overview.mockRejectedValue(new ApiError(500, "internal", "Something went wrong on the server."));
    render(<OverviewPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent(/something went wrong/i);
  });
});
