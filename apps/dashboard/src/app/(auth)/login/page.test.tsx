import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/errors";
import LoginPage from "./page";

const push = vi.fn();
const refresh = vi.fn();
const login = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, refresh }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => "/login",
}));

vi.mock("@/lib/api", () => ({
  api: {
    login: (...args: unknown[]) => login(...args),
  },
}));

describe("login", () => {
  beforeEach(() => {
    push.mockReset();
    refresh.mockReset();
    login.mockReset();
  });

  it("signs in with valid credentials", async () => {
    login.mockResolvedValue({ user_id: "u1", email: "a@b.com" });
    const user = userEvent.setup();
    render(<LoginPage />);
    await user.type(screen.getByLabelText(/email/i), "owner@example.com");
    await user.type(screen.getByLabelText(/password/i), "supersecret");
    await user.click(screen.getByRole("button", { name: /sign in/i }));
    expect(login).toHaveBeenCalledWith("owner@example.com", "supersecret");
    expect(push).toHaveBeenCalledWith("/");
  });

  it("shows an error when credentials are rejected", async () => {
    login.mockRejectedValue(new ApiError(401, "unauthorized", "Your session expired. Sign in again."));
    const user = userEvent.setup();
    render(<LoginPage />);
    await user.type(screen.getByLabelText(/email/i), "owner@example.com");
    await user.type(screen.getByLabelText(/password/i), "wrong-password");
    await user.click(screen.getByRole("button", { name: /sign in/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/session expired|invalid|failed/i);
    expect(push).not.toHaveBeenCalled();
  });
});
