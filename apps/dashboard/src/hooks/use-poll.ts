"use client";

import { useEffect } from "react";

export function usePoll(ms: number, fn: () => void) {
  useEffect(() => {
    if (!ms) return;
    const id = setInterval(fn, ms);
    return () => clearInterval(id);
  }, [ms, fn]);
}
