"use client";

import { Shell } from "@/components/shell";
import { useApp } from "@/components/providers";
import { Skeleton } from "@/components/ui";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { loading } = useApp();
  if (loading) {
    return (
      <div className="flex min-h-screen">
        <div className="w-56 border-r border-border p-4">
          <Skeleton className="h-6 w-24" />
          <Skeleton className="mt-6 h-8 w-full" />
          <Skeleton className="mt-8 h-4 w-20" />
        </div>
        <div className="flex-1 p-6">
          <Skeleton className="h-8 w-48" />
          <div className="mt-6 grid grid-cols-4 gap-3">
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
          </div>
        </div>
      </div>
    );
  }
  return <Shell>{children}</Shell>;
}
