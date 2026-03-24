/**
 * SystemSelector.jsx
 *
 * Displays a card grid of available time-reporting systems.
 * The user selects one or more systems to sync with before proceeding
 * to the authentication step.
 *
 * Adding a new system on the frontend:
 * ─────────────────────────────────────
 * No changes needed here – systems are fetched dynamically from GET /api/systems.
 * The system's name, description, and auth fields all come from the backend.
 * Optionally add an icon mapping in SYSTEM_ICONS below.
 */

import React from "react";

/**
 * Optional icon overrides keyed by systemId.
 * Add an entry here when you register a new adapter to get a nice icon.
 */
const SYSTEM_ICONS = {
  maconomy: (
    <svg
      viewBox="0 0 40 40"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <rect width="40" height="40" rx="8" fill="#1a3a5c" />
      <text
        x="50%"
        y="56%"
        dominantBaseline="middle"
        textAnchor="middle"
        fill="white"
        fontSize="14"
        fontWeight="bold"
        fontFamily="monospace"
      >
        M
      </text>
    </svg>
  ),
  fieldglass: (
    <svg
      viewBox="0 0 40 40"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <rect width="40" height="40" rx="8" fill="#0070c0" />
      <text
        x="50%"
        y="56%"
        dominantBaseline="middle"
        textAnchor="middle"
        fill="white"
        fontSize="14"
        fontWeight="bold"
        fontFamily="monospace"
      >
        FG
      </text>
    </svg>
  ),
};

/** Fallback icon for unknown systems */
function DefaultIcon({ name }) {
  const initials = name
    .split(" ")
    .slice(0, 2)
    .map((w) => w[0])
    .join("")
    .toUpperCase();
  return (
    <svg
      viewBox="0 0 40 40"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <rect width="40" height="40" rx="8" fill="#4b5563" />
      <text
        x="50%"
        y="56%"
        dominantBaseline="middle"
        textAnchor="middle"
        fill="white"
        fontSize="13"
        fontWeight="bold"
        fontFamily="monospace"
      >
        {initials}
      </text>
    </svg>
  );
}

/**
 * SystemSelector
 *
 * @param {Object}   props
 * @param {SystemInfo[]} props.systems          - All available systems
 * @param {string[]}     props.selectedIds      - Currently selected system IDs
 * @param {function}     props.onToggle         - Called with systemId when card is clicked
 */
export default function SystemSelector({ systems, selectedIds, onToggle }) {
  if (!systems || systems.length === 0) {
    return (
      <p className="text-slate-400 text-sm text-center py-8">
        No systems available. Check that the backend is running.
      </p>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      {systems.map((system) => {
        const isSelected = selectedIds.includes(system.id);
        return (
          <button
            key={system.id}
            onClick={() => onToggle(system.id)}
            aria-pressed={isSelected}
            className={[
              "system-card",
              "relative flex items-start gap-4 p-5 rounded-xl border-2 text-left",
              "transition-all duration-200 cursor-pointer",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
              isSelected
                ? "border-amber-400 bg-amber-400/5 shadow-lg shadow-amber-400/10"
                : "border-slate-700 bg-slate-800/60 hover:border-slate-500 hover:bg-slate-800",
            ].join(" ")}
          >
            {/* Selection checkmark */}
            <span
              className={[
                "absolute top-3 right-3 w-5 h-5 rounded-full border-2 flex items-center justify-center",
                "transition-all duration-200",
                isSelected
                  ? "border-amber-400 bg-amber-400"
                  : "border-slate-600 bg-transparent",
              ].join(" ")}
              aria-hidden="true"
            >
              {isSelected && (
                <svg
                  className="w-3 h-3 text-slate-900"
                  fill="currentColor"
                  viewBox="0 0 12 12"
                >
                  <path
                    d="M10 3L5 8.5 2 5.5"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    fill="none"
                  />
                </svg>
              )}
            </span>

            {/* Icon */}
            <span className="flex-shrink-0 w-10 h-10">
              {SYSTEM_ICONS[system.id] || <DefaultIcon name={system.name} />}
            </span>

            {/* Text */}
            <span className="flex-1 min-w-0">
              <span className="block font-semibold text-slate-100 text-sm leading-snug">
                {system.name}
              </span>
              <span className="block text-slate-400 text-xs mt-1 leading-relaxed">
                {system.description}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
