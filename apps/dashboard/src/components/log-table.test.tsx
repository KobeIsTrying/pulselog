import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { sampleLog } from "@/test/fixtures";
import { EmptyState, ErrorBanner, LevelBadge } from "./ui";
import { LogTable } from "./log-table";

describe("log table", () => {
  it("renders timestamp, level, service, and message", () => {
    render(<LogTable logs={[sampleLog]} onSelect={vi.fn()} />);
    expect(screen.getAllByText("ERROR").length).toBeGreaterThan(0);
    expect(screen.getAllByText("payment-service").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Payment authorization failed").length).toBeGreaterThan(0);
  });

  it("selects a row", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<LogTable logs={[sampleLog]} onSelect={onSelect} />);
    await user.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalledWith(sampleLog);
  });
});

describe("empty and error states", () => {
  it("guides the user when there are no logs", () => {
    render(<EmptyState title="No logs yet" body="Create an API key and send your first event." />);
    expect(screen.getByText(/no logs yet/i)).toBeInTheDocument();
    expect(screen.getByText(/api key/i)).toBeInTheDocument();
  });

  it("surfaces API failures", () => {
    render(<ErrorBanner message="The query service is unavailable. Check that query-api and ClickHouse are running." />);
    expect(screen.getByRole("alert")).toHaveTextContent(/unavailable/i);
  });

  it("does not rely only on color for severity", () => {
    render(<LevelBadge level="ERROR" />);
    expect(screen.getAllByText("ERROR").length).toBeGreaterThan(0);
  });
});
