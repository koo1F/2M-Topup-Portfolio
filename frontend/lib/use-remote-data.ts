"use client";
import { useCallback, useEffect, useState } from "react";

// Request identity prevents a stale response from replacing a newer page.
export function useRemoteData<T>(url: string) {
  const [version, setVersion] = useState(0);
  const key = `${url}:${version}`;
  const [result, setResult] = useState<{ key: string; data?: T; error?: string }>();
  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const response = await fetch(url, { signal: controller.signal });
        if (!response.ok) throw new Error(`Request failed (${response.status})`);
        const data: T = await response.json();
        if (!controller.signal.aborted) setResult({ key, data });
      } catch (error) {
        if (!controller.signal.aborted) setResult({ key, error: error instanceof Error ? error.message : "Request failed" });
      }
    }
    void load();
    return () => controller.abort();
  }, [url, key]);
  const refresh = useCallback(() => setVersion((value) => value + 1), []);
  const current = result?.key === key ? result : undefined;
  return { data: current?.data, error: current?.error ?? "", loading: !current, refresh };
}
