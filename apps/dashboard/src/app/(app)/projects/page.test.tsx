import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { sessionUser } from "@/test/fixtures";
import ProjectsPage from "./page";

const setProjectId = vi.fn();

vi.mock("@/components/providers", () => ({
  useApp: () => ({
    user: sessionUser,
    org: sessionUser.orgs[0],
    role: "owner",
    project: { id: "p1", org_id: "org", name: "default", slug: "default" },
    projects: [
      { id: "p1", org_id: "org", name: "default", slug: "default" },
      { id: "p2", org_id: "org", name: "staging", slug: "staging" },
    ],
    setProjectId,
    refreshSession: vi.fn(),
  }),
}));

describe("project switching", () => {
  it("switches the active project", async () => {
    const user = userEvent.setup();
    render(<ProjectsPage />);
    await user.click(screen.getByRole("button", { name: /switch/i }));
    expect(setProjectId).toHaveBeenCalledWith("p2");
  });
});
