"use client";

import { api } from "@/lib/api";
import { ApiError } from "@/lib/errors";
import type { OrgMembership, Project, Role, SessionUser, TimeRangeKey } from "@/lib/types";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { LiveProvider } from "./live-provider";

type PollMs = 0 | 10000 | 30000 | 60000;

interface AppState {
  user: SessionUser | null;
  loading: boolean;
  org: OrgMembership | null;
  project: Project | null;
  projects: Project[];
  role: Role | undefined;
  range: TimeRangeKey;
  pollMs: PollMs;
  setRange: (r: TimeRangeKey) => void;
  setPollMs: (n: PollMs) => void;
  setProjectId: (id: string) => void;
  refreshSession: () => Promise<void>;
  logout: () => Promise<void>;
}

const Ctx = createContext<AppState | null>(null);

export function useApp() {
  const v = useContext(Ctx);
  if (!v) throw new Error("useApp requires providers");
  return v;
}

export function AppProviders({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<SessionUser | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectId, setProjectId] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [range, setRange] = useState<TimeRangeKey>("24h");
  const [pollMs, setPollMs] = useState<PollMs>(0);

  const refreshSession = useCallback(async () => {
    try {
      const me = await api.session();
      setUser(me);
      const firstOrg = me.orgs?.[0];
      if (firstOrg) {
        const { projects: list } = await api.projects(firstOrg.org.id);
        setProjects(list || []);
        setProjectId((cur) => {
          if (cur && list.some((p) => p.id === cur)) return cur;
          const stored = typeof window !== "undefined" ? sessionStorage.getItem("pulselog.project") : "";
          if (stored && list.some((p) => p.id === stored)) return stored;
          return list[0]?.id || "";
        });
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 401 && pathname !== "/login" && pathname !== "/signup") {
        router.push("/login");
      }
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, [pathname, router]);

  useEffect(() => {
    if (pathname === "/login" || pathname === "/signup") {
      setLoading(false);
      return;
    }
    void refreshSession();
  }, [refreshSession, pathname]);

  useEffect(() => {
    if (projectId) sessionStorage.setItem("pulselog.project", projectId);
  }, [projectId]);

  const org = user?.orgs?.[0] ?? null;
  const project = projects.find((p) => p.id === projectId) ?? null;

  const logout = useCallback(async () => {
    await api.logout().catch(() => undefined);
    setUser(null);
    router.push("/login");
  }, [router]);

  const value = useMemo<AppState>(
    () => ({
      user,
      loading,
      org,
      project,
      projects,
      role: org?.role,
      range,
      pollMs,
      setRange,
      setPollMs,
      setProjectId,
      refreshSession,
      logout,
    }),
    [user, loading, org, project, projects, range, pollMs, refreshSession, logout],
  );

  return (
    <Ctx.Provider value={value}>
      <LiveProvider>{children}</LiveProvider>
    </Ctx.Provider>
  );
}
