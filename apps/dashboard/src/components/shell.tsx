"use client";

import { cn } from "@/lib/cn";
import { LiveIndicator } from "./live-bar";
import { useApp } from "./providers";
import { Activity, KeyRound, Layers3, LayoutDashboard, LogOut, Menu, ScrollText, Settings, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

const nav = [
  { href: "/", label: "Overview", icon: LayoutDashboard },
  { href: "/logs", label: "Logs", icon: ScrollText },
  { href: "/services", label: "Services", icon: Activity },
];

const manage = [
  { href: "/projects", label: "Projects", icon: Layers3 },
  { href: "/api-keys", label: "API Keys", icon: KeyRound },
];

export function Shell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { user, org, project, projects, setProjectId, logout } = useApp();
  const [open, setOpen] = useState(false);

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-raised max-md:hidden">
        <div className="border-b border-border px-4 py-4">
          <Link href="/" className="flex items-center gap-2 text-sm font-semibold tracking-tight">
            <span className="inline-flex h-6 w-6 items-center justify-center rounded bg-accent text-xs font-bold text-black">P</span>
            PulseLog
          </Link>
          <p className="mt-3 truncate text-[11px] uppercase tracking-wider text-muted">{org?.org.name || "Organization"}</p>
          <label className="sr-only" htmlFor="project-select">
            Project
          </label>
          <select
            id="project-select"
            className="mt-1 w-full rounded-md border border-border bg-bg px-2 py-1.5 text-xs"
            value={project?.id || ""}
            onChange={(e) => setProjectId(e.target.value)}
          >
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </div>
        <nav className="flex-1 space-y-4 px-2 py-3" aria-label="Main">
          <NavGroup title="Monitor" items={nav} pathname={pathname} />
          <NavGroup title="Management" items={manage} pathname={pathname} />
          <NavGroup title="System" items={[{ href: "/settings", label: "Settings", icon: Settings }]} pathname={pathname} />
        </nav>
        <div className="border-t border-border px-3 py-3">
          <LiveIndicator />
          <p className="truncate text-xs text-ink">{user?.email}</p>
          <p className="text-[11px] capitalize text-muted">{org?.role}</p>
          <button
            type="button"
            onClick={() => void logout()}
            className="mt-2 inline-flex items-center gap-1.5 text-xs text-muted hover:text-ink"
          >
            <LogOut className="h-3.5 w-3.5" aria-hidden />
            Logout
          </button>
        </div>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between border-b border-border px-4 py-3 md:hidden">
          <span className="text-sm font-semibold">PulseLog</span>
          <button type="button" className="rounded-md p-1 text-muted hover:bg-hover hover:text-ink" aria-expanded={open} aria-label="Open navigation" onClick={() => setOpen(true)}>
            <Menu className="h-5 w-5" aria-hidden />
          </button>
        </header>
        {open ? (
          <div className="fixed inset-0 z-30 md:hidden">
            <button type="button" className="absolute inset-0 bg-black/50" aria-label="Close navigation" onClick={() => setOpen(false)} />
            <div className="relative flex h-full w-64 flex-col border-r border-border bg-raised">
              <div className="flex items-center justify-between border-b border-border px-4 py-3">
                <span className="text-sm font-semibold">PulseLog</span>
                <button type="button" aria-label="Close navigation" onClick={() => setOpen(false)}>
                  <X className="h-5 w-5" />
                </button>
              </div>
              <div className="px-3 py-2">
                <label className="sr-only" htmlFor="project-select-mobile">
                  Project
                </label>
                <select
                  id="project-select-mobile"
                  className="w-full rounded-md border border-border bg-bg px-2 py-1.5 text-xs"
                  value={project?.id || ""}
                  onChange={(e) => setProjectId(e.target.value)}
                >
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>
              <nav className="flex-1 space-y-4 px-2 py-2" onClick={() => setOpen(false)}>
                <NavGroup title="Monitor" items={nav} pathname={pathname} />
                <NavGroup title="Management" items={manage} pathname={pathname} />
                <NavGroup title="System" items={[{ href: "/settings", label: "Settings", icon: Settings }]} pathname={pathname} />
              </nav>
              <div className="border-t border-border px-3 py-3">
                <p className="truncate text-xs">{user?.email}</p>
                <button type="button" onClick={() => void logout()} className="mt-2 text-xs text-muted">
                  Logout
                </button>
              </div>
            </div>
          </div>
        ) : null}
        <main className="flex-1 p-4 md:p-6">{children}</main>
      </div>
    </div>
  );
}

function NavGroup({
  title,
  items,
  pathname,
}: {
  title: string;
  items: { href: string; label: string; icon: typeof LayoutDashboard }[];
  pathname: string;
}) {
  return (
    <div>
      <p className="px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted">{title}</p>
      {items.map((item) => {
        const active = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
        const Icon = item.icon;
        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm",
              active ? "bg-hover text-ink" : "text-muted hover:bg-hover hover:text-ink",
            )}
            aria-current={active ? "page" : undefined}
          >
            <Icon className="h-4 w-4" aria-hidden />
            {item.label}
          </Link>
        );
      })}
    </div>
  );
}
