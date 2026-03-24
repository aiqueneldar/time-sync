/**
 * SyncStatus.jsx
 *
 * Displays real-time synchronisation status chips for each system.
 * Chips cycle through:
 *   pending  → grey
 *   syncing  → amber/pulsing
 *   synced   → green
 *   error    → red (expandable details)
 *
 * Receives status data via props fed from the SSE stream in TimesheetPage.
 */

import React, { useState } from "react";

/** Status → visual config mapping */
const STATUS_CONFIG = {
  pending: {
    bg: "bg-slate-700/60",
    border: "border-slate-600",
    text: "text-slate-400",
    dot: "bg-slate-500",
    label: "Pending",
    pulse: false,
  },
  syncing: {
    bg: "bg-amber-400/10",
    border: "border-amber-400/50",
    text: "text-amber-300",
    dot: "bg-amber-400",
    label: "Syncing",
    pulse: true,
  },
  synced: {
    bg: "bg-emerald-400/10",
    border: "border-emerald-500/50",
    text: "text-emerald-300",
    dot: "bg-emerald-400",
    label: "Synced",
    pulse: false,
  },
  error: {
    bg: "bg-red-400/10",
    border: "border-red-500/50",
    text: "text-red-300",
    dot: "bg-red-400",
    label: "Error",
    pulse: false,
  },
};

/**
 * SyncChip displays the status for one system.
 *
 * @param {Object}     props
 * @param {string}     props.systemName
 * @param {SyncStatus} props.status
 */
function SyncChip({ systemName, status }) {
  const [expanded, setExpanded] = useState(false);
  const cfg = STATUS_CONFIG[status?.state] || STATUS_CONFIG.pending;

  return (
    <div className="inline-flex flex-col">
      <button
        onClick={() => (status?.message ? setExpanded((v) => !v) : undefined)}
        className={[
          "inline-flex items-center gap-2 px-3 py-1.5 rounded-full border",
          "text-xs font-medium transition-all duration-300",
          cfg.bg,
          cfg.border,
          cfg.text,
          status?.message
            ? "cursor-pointer hover:opacity-90"
            : "cursor-default",
          "focus:outline-none focus-visible:ring-1 focus-visible:ring-amber-400",
        ].join(" ")}
        title={status?.message || ""}
        aria-expanded={status?.message ? expanded : undefined}
      >
        {/* Status dot */}
        <span
          className={[
            "w-2 h-2 rounded-full flex-shrink-0",
            cfg.dot,
            cfg.pulse ? "animate-pulse" : "",
          ].join(" ")}
          aria-hidden="true"
        />

        <span>{systemName}</span>
        <span className="opacity-70">·</span>
        <span>{cfg.label}</span>

        {/* Expand arrow for messages */}
        {status?.message && (
          <svg
            className={`w-3 h-3 transition-transform ${expanded ? "rotate-180" : ""}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        )}
      </button>

      {/* Expanded message */}
      {expanded && status?.message && (
        <div
          className={[
            "mt-1.5 px-3 py-2 rounded-lg text-xs border max-w-xs",
            cfg.bg,
            cfg.border,
            cfg.text,
          ].join(" ")}
        >
          {status.message}
        </div>
      )}
    </div>
  );
}

/**
 * SyncStatusBar renders the row of sync-status chips for all selected systems.
 *
 * @param {Object}               props
 * @param {SystemInfo[]}         props.systems       - The selected systems
 * @param {Record<id,SyncStatus>} props.syncStatuses  - Current statuses
 */
export default function SyncStatusBar({ systems, syncStatuses }) {
  if (!systems || systems.length === 0) return null;

  return (
    <div
      className="flex flex-wrap items-center gap-2"
      role="status"
      aria-live="polite"
      aria-label="Sync status"
    >
      {systems.map((system) => (
        <SyncChip
          key={system.id}
          systemName={system.name}
          status={syncStatuses?.[system.id]}
        />
      ))}
    </div>
  );
}
