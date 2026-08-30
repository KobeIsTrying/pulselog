import { cn } from "@/lib/cn";
import type { LogLevel } from "@/lib/types";
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from "react";

export function Button({
  className,
  variant = "primary",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "ghost" | "danger" | "outline" }) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50",
        variant === "primary" && "bg-accent text-black hover:bg-[#62b0f5]",
        variant === "ghost" && "text-muted hover:bg-hover hover:text-ink",
        variant === "danger" && "bg-error/15 text-error hover:bg-error/25",
        variant === "outline" && "border border-border bg-raised text-ink hover:bg-hover",
        className,
      )}
      {...props}
    />
  );
}

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "w-full rounded-md border border-border bg-bg px-3 py-2 text-sm text-ink placeholder:text-muted",
        className,
      )}
      {...props}
    />
  );
}

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <section className={cn("rounded-lg border border-border bg-raised", className)}>{children}</section>;
}

export function Label({ children, htmlFor }: { children: ReactNode; htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="mb-1 block text-xs font-medium uppercase tracking-wide text-muted">
      {children}
    </label>
  );
}

const levelStyles: Record<string, string> = {
  ERROR: "bg-error/15 text-error ring-error/30",
  FATAL: "bg-error/20 text-error ring-error/40",
  WARN: "bg-warn/15 text-warn ring-warn/30",
  INFO: "bg-info/15 text-info ring-info/30",
  DEBUG: "bg-white/5 text-muted ring-white/10",
};

export function LevelBadge({ level }: { level: LogLevel | string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-1.5 py-0.5 font-mono text-[11px] font-semibold uppercase ring-1",
        levelStyles[level] || levelStyles.DEBUG,
      )}
    >
      {level}
    </span>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-md bg-hover", className)} />;
}

export function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="px-6 py-12 text-center">
      <p className="text-sm font-medium text-ink">{title}</p>
      <p className="mt-1 text-sm text-muted">{body}</p>
    </div>
  );
}

export function ErrorBanner({ message }: { message: string }) {
  return (
    <div role="alert" className="rounded-md border border-error/30 bg-error/10 px-3 py-2 text-sm text-error">
      {message}
    </div>
  );
}
