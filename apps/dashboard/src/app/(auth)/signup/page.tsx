"use client";

import { api } from "@/lib/api";
import { ApiError } from "@/lib/errors";
import { Button, ErrorBanner, Input, Label } from "@/components/ui";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";

export default function SignupPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [organization, setOrganization] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (!email.includes("@") || password.length < 10 || !organization.trim()) {
      setError("Email, organization, and a password of at least 10 characters are required.");
      return;
    }
    setLoading(true);
    try {
      await api.register(email, password, organization.trim());
      router.push("/");
      router.refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the account.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <p className="text-xs uppercase tracking-[0.2em] text-muted">PulseLog</p>
        <h1 className="mt-2 text-2xl font-semibold">Create account</h1>
        <p className="mt-1 mb-6 text-sm text-muted">Creates an organization, a default project, and an owner role.</p>
        <form onSubmit={onSubmit} className="space-y-4">
          {error ? <ErrorBanner message={error} /> : null}
          <div>
            <Label htmlFor="org">Organization</Label>
            <Input id="org" value={organization} onChange={(e) => setOrganization(e.target.value)} required />
          </div>
          <div>
            <Label htmlFor="email">Email</Label>
            <Input id="email" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div>
            <Label htmlFor="password">Password</Label>
            <Input id="password" type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Creating…" : "Create account"}
          </Button>
        </form>
        <p className="mt-4 text-sm text-muted">
          Already registered?{" "}
          <Link href="/login" className="text-accent hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
