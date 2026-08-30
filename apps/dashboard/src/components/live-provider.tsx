"use client";

import { api } from "@/lib/api";
import { nextReconnectDelay, parseLiveFrame, resolveWsBase, streamUrl, type LiveStatus } from "@/lib/live";
import type { LogEvent } from "@/lib/types";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useApp } from "./providers";

type Listener = (events: LogEvent[]) => void;

interface LiveState {
  enabled: boolean;
  setEnabled: (on: boolean) => void;
  status: LiveStatus;
  paused: boolean;
  setPaused: (on: boolean) => void;
  pending: number;
  flushPending: () => LogEvent[];
  subscribe: (fn: Listener) => () => void;
}

const disabled: LiveState = {
  enabled: false,
  setEnabled: () => undefined,
  status: "disconnected",
  paused: false,
  setPaused: () => undefined,
  pending: 0,
  flushPending: () => [],
  subscribe: () => () => undefined,
};

const Ctx = createContext<LiveState | null>(null);

export function useLive(): LiveState {
  return useContext(Ctx) ?? disabled;
}

export function LiveProvider({ children }: { children: ReactNode }) {
  const { project, user } = useApp();
  const [enabled, setEnabled] = useState(false);
  const [status, setStatus] = useState<LiveStatus>("disconnected");
  const [paused, setPaused] = useState(false);
  const [pending, setPending] = useState(0);
  const listeners = useRef(new Set<Listener>());
  const buffer = useRef<LogEvent[]>([]);
  const batch = useRef<LogEvent[]>([]);
  const flushTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pausedRef = useRef(false);
  const wsRef = useRef<WebSocket | null>(null);
  const backoffRef = useRef(0);
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const generation = useRef(0);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  useEffect(() => {
    buffer.current = [];
    batch.current = [];
    setPending(0);
    setPaused(false);
  }, [project?.id]);

  const emit = useCallback((events: LogEvent[]) => {
    if (!events.length) return;
    if (pausedRef.current) {
      buffer.current.push(...events);
      setPending(buffer.current.length);
      return;
    }
    listeners.current.forEach((fn) => fn(events));
  }, []);

  const queue = useCallback(
    (events: LogEvent[]) => {
      batch.current.push(...events);
      if (flushTimer.current) return;
      flushTimer.current = setTimeout(() => {
        flushTimer.current = null;
        const next = batch.current;
        batch.current = [];
        emit(next);
      }, 200);
    },
    [emit],
  );

  const flushPending = useCallback(() => {
    const held = buffer.current;
    buffer.current = [];
    setPending(0);
    if (held.length) listeners.current.forEach((fn) => fn(held));
    return held;
  }, []);

  const subscribe = useCallback((fn: Listener) => {
    listeners.current.add(fn);
    return () => {
      listeners.current.delete(fn);
    };
  }, []);

  const setPausedSafe = useCallback(
    (on: boolean) => {
      setPaused(on);
      pausedRef.current = on;
      if (!on) flushPending();
    },
    [flushPending],
  );

  useEffect(() => {
    if (!enabled || !project?.id || !user) {
      generation.current += 1;
      if (retryRef.current) clearTimeout(retryRef.current);
      wsRef.current?.close();
      wsRef.current = null;
      setStatus("disconnected");
      return;
    }

    let stopped = false;
    const gen = ++generation.current;

    const connect = async () => {
      if (stopped || generation.current !== gen) return;
      setStatus(backoffRef.current > 0 ? "reconnecting" : "reconnecting");
      try {
        const [{ ticket }, wsBase] = await Promise.all([api.streamTicket(), resolveWsBase()]);
        if (stopped || generation.current !== gen) return;
        const ws = new WebSocket(streamUrl(ticket, project.id, wsBase));
        wsRef.current = ws;
        ws.onopen = () => {
          if (generation.current !== gen) {
            ws.close();
            return;
          }
          backoffRef.current = 0;
          setStatus("live");
        };
        ws.onmessage = (ev) => {
          const log = parseLiveFrame(typeof ev.data === "string" ? ev.data : "");
          if (log) queue([log]);
        };
        ws.onerror = () => undefined;
        ws.onclose = () => {
          if (stopped || generation.current !== gen) return;
          setStatus("reconnecting");
          const wait = nextReconnectDelay(backoffRef.current);
          backoffRef.current = wait;
          retryRef.current = setTimeout(() => void connect(), wait);
        };
      } catch {
        if (stopped || generation.current !== gen) return;
        setStatus("reconnecting");
        const wait = nextReconnectDelay(backoffRef.current);
        backoffRef.current = wait;
        retryRef.current = setTimeout(() => void connect(), wait);
      }
    };

    void connect();
    return () => {
      stopped = true;
      if (retryRef.current) clearTimeout(retryRef.current);
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [enabled, project?.id, user, queue]);

  useEffect(() => {
    return () => {
      if (flushTimer.current) clearTimeout(flushTimer.current);
    };
  }, []);

  const value = useMemo<LiveState>(
    () => ({
      enabled,
      setEnabled: (on) => {
        setEnabled(on);
        if (!on) {
          setPaused(false);
          buffer.current = [];
          setPending(0);
        }
      },
      status,
      paused,
      setPaused: setPausedSafe,
      pending,
      flushPending,
      subscribe,
    }),
    [enabled, status, paused, pending, setPausedSafe, flushPending, subscribe],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
