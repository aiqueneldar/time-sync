/**
 * RowMapper.jsx
 *
 * Lets the user map one frontend time-entry row to one row in each
 * authenticated backend system.
 *
 * Displays a collapsible panel with a dropdown per system.
 * The mapping is stored in the row's `mappings` array.
 */

import React, { useState } from "react";

/**
 * RowMapper
 *
 * @param {Object}               props
 * @param {TimeEntryRow}         props.row             - The row being mapped
 * @param {SystemInfo[]}         props.systems         - Authenticated systems
 * @param {Record<id,SystemRow[]>} props.availableRows  - Backend rows per system
 * @param {function}             props.onChange        - Called with new mappings array
 */
export default function RowMapper({ row, systems, availableRows, onChange }) {
  const [open, setOpen] = useState(false);

  const mappingCount = row.mappings?.length || 0;

  function handleSelect(systemId, systemRowId) {
    const existing = (row.mappings || []).filter(
      (m) => m.systemId !== systemId,
    );
    if (systemRowId === "") {
      onChange(existing);
    } else {
      onChange([...existing, { systemId, systemRowId }]);
    }
  }

  function getMappedRowId(systemId) {
    return (
      row.mappings?.find((m) => m.systemId === systemId)?.systemRowId || ""
    );
  }

  return (
    <div className="mt-2">
      {/* Toggle button */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className={[
          "flex items-center gap-2 text-xs px-3 py-1.5 rounded-lg border transition-all",
          "focus:outline-none focus-visible:ring-1 focus-visible:ring-amber-400",
          open || mappingCount > 0
            ? "border-amber-400/40 bg-amber-400/5 text-amber-300"
            : "border-slate-600 bg-slate-800 text-slate-400 hover:border-slate-500 hover:text-slate-300",
        ].join(" ")}
        aria-expanded={open}
      >
        <svg
          className="w-3.5 h-3.5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
          />
        </svg>
        {mappingCount > 0
          ? `${mappingCount} mapping${mappingCount > 1 ? "s" : ""}`
          : "Map to system rows"}
        <svg
          className={`w-3 h-3 transition-transform ${open ? "rotate-180" : ""}`}
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
      </button>

      {/* Mapping selectors */}
      {open && (
        <div className="mt-2 p-4 rounded-xl border border-slate-700 bg-slate-800/80 space-y-3">
          <p className="text-xs text-slate-400 mb-3">
            Select which row in each connected system to book this time against.
          </p>

          {systems.length === 0 && (
            <p className="text-xs text-slate-500 italic">
              No authenticated systems. Complete setup first.
            </p>
          )}

          {systems.map((system) => {
            const rows = availableRows?.[system.id] || [];
            const currentValue = getMappedRowId(system.id);

            return (
              <div key={system.id} className="flex items-center gap-3">
                <label
                  htmlFor={`map-${row.id}-${system.id}`}
                  className="text-xs text-slate-400 w-28 flex-shrink-0 font-medium"
                >
                  {system.name}
                </label>

                <div className="flex-1 relative">
                  {rows.length === 0 ? (
                    <span className="text-xs text-slate-600 italic">
                      No rows available – check auth
                    </span>
                  ) : (
                    <select
                      id={`map-${row.id}-${system.id}`}
                      value={currentValue}
                      onChange={(e) => handleSelect(system.id, e.target.value)}
                      className={[
                        "w-full px-3 py-2 rounded-lg text-xs",
                        "bg-slate-700 border border-slate-600",
                        "text-slate-200",
                        "focus:outline-none focus:border-amber-400",
                        "transition-colors appearance-none cursor-pointer",
                      ].join(" ")}
                    >
                      <option value="">— not mapped —</option>
                      {rows.map((sysRow) => (
                        <option key={sysRow.id} value={sysRow.id}>
                          {sysRow.code ? `[${sysRow.code}] ` : ""}
                          {sysRow.name}
                        </option>
                      ))}
                    </select>
                  )}
                </div>

                {/* Clear mapping button */}
                {currentValue && (
                  <button
                    type="button"
                    onClick={() => handleSelect(system.id, "")}
                    className="text-slate-500 hover:text-red-400 transition-colors p-1 rounded
                      focus:outline-none focus-visible:ring-1 focus-visible:ring-amber-400"
                    aria-label={`Remove ${system.name} mapping`}
                  >
                    <svg
                      className="w-3.5 h-3.5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M6 18L18 6M6 6l12 12"
                      />
                    </svg>
                  </button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
