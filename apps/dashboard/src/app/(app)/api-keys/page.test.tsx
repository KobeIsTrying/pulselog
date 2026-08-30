import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { sessionUser } from "@/test/fixtures";
import ApiKeysPage from "./page";

const api = {
  apiKeys: vi.fn(),
  services: vi.fn(),
  createApiKey: vi.fn(),
  revokeApiKey: vi.fn(),
};

vi.mock("@/lib/api", () => ({
  api: {
    apiKeys: (...a: unknown[]) => api.apiKeys(...a),
    services: (...a: unknown[]) => api.services(...a),
    createApiKey: (...a: unknown[]) => api.createApiKey(...a),
    revokeApiKey: (...a: unknown[]) => api.revokeApiKey(...a),
  },
}));

const ctx = vi.hoisted(() => ({ role: "owner" as "owner" | "viewer" }));

vi.mock("@/components/providers", () => ({
  useApp: () => ({
    user: sessionUser,
    role: ctx.role,
    project: { id: "proj-1", org_id: "org", name: "default", slug: "default" },
  }),
}));

describe("API key management", () => {
  beforeEach(() => {
    ctx.role = "owner";
    api.apiKeys.mockReset();
    api.services.mockReset();
    api.createApiKey.mockReset();
    api.revokeApiKey.mockReset();
    api.apiKeys.mockResolvedValue({
      keys: [
        {
          id: "key-1",
          project_id: "proj-1",
          service_id: "svc-1",
          service: "payment-service",
          name: "ci",
          prefix: "pl_live_ab",
          created_at: "2026-08-29T12:00:00.000Z",
        },
      ],
    });
    api.services.mockResolvedValue({ services: [{ id: "svc-1", project_id: "proj-1", name: "payment-service" }] });
    api.createApiKey.mockResolvedValue({
      id: "key-2",
      prefix: "pl_live_cd",
      name: "local",
      service: "payment-service",
      project_id: "proj-1",
      token: "pl_live_only-once",
    });
    api.revokeApiKey.mockResolvedValue({ status: "revoked" });
  });

  it("creates a key and shows the secret once", async () => {
    const user = userEvent.setup();
    render(<ApiKeysPage />);
    await screen.findByText("ci");
    await user.type(screen.getByLabelText(/^name$/i), "local");
    await user.click(screen.getByRole("button", { name: /create key/i }));
    expect(api.createApiKey).toHaveBeenCalledWith("proj-1", "local", "payment-service");
    expect(await screen.findByText("pl_live_only-once")).toBeInTheDocument();
    expect(screen.getByText(/will not be shown again/i)).toBeInTheDocument();
  });

  it("revokes a key", async () => {
    vi.stubGlobal("confirm", () => true);
    const user = userEvent.setup();
    render(<ApiKeysPage />);
    await screen.findByText("ci");
    await user.click(screen.getByRole("button", { name: /revoke/i }));
    expect(api.revokeApiKey).toHaveBeenCalledWith("key-1");
  });

  it("blocks viewers from managing keys", () => {
    ctx.role = "viewer";
    render(<ApiKeysPage />);
    expect(screen.getByRole("alert")).toHaveTextContent(/permission/i);
    expect(api.apiKeys).not.toHaveBeenCalled();
  });
});
