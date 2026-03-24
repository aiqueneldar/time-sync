/**
 * TimeTable.jsx
 *
 * The core time-entry grid.
 *
 * Layout:
 *   Row label | Mon | Tue | Wed | Thu | Fri | Sat | Sun | Total | [map] [delete]
 *
 * Each hour input is a controlled number field.
 * Rows can be added and removed dynamically.
 * Each row has an expandable RowMapper for mapping to backend system rows.
 */

import React, { useMemo } from "react";
import RowMapper from "../RowMapper/RowMapper";

const DAYS = [
  "monday",
  "tuesday",
  "wednesday",
  "thursday",
  "friday",
  "saturday",
  "sunday",
];
const DAY_LABELS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

/**
 * Get the Monday Date object for the given ISO year/week.
 * @param {number} year
 * @param {number} week
 * @returns {Date}
 */
function weekStart(year, week) {
  const jan4 = new Date(year, 0, 4);
  const day = jan4.getDay() || 7; // Mon=1 … Sun=7
  const mon1 = new Date(jan4);
  mon1.setDate(jan4.getDate() - (day - 1));
  const result = new Date(mon1);
  result.setDate(mon1.getDate() + (week - 1) * 7);
  return result;
}

/**
 * TimeTable
 *
 * @param {Object}               props
 * @param {TimeEntryRow[]}       props.rows
 * @param {{ year, week }}       props.week
 * @param {SystemInfo[]}         props.systems         - Authenticated systems
 * @param {Record<id,SystemRow[]>} props.availableRows
 * @param {function}             props.onLabelChange   - (rowId, label) => void
 * @param {function}             props.onHoursChange   - (rowId, day, value) => void
 * @param {function}             props.onMappingsChange- (rowId, mappings) => void
 * @param {function}             props.onAddRow        - () => void
 * @param {function}             props.onRemoveRow     - (rowId) => void
 */
export default function TimeTable({
  rows,
  week,
  systems,
  availableRows,
  onLabelChange,
  onHoursChange,
  onMappingsChange,
  onAddRow,
  onRemoveRow,
}) {
  // Compute the Date for each column header.
  const columnDates = useMemo(() => {
    const monday = weekStart(week.year, week.week);
    return DAYS.map((_, i) => {
      const d = new Date(monday);
      d.setDate(monday.getDate() + i);
      return d;
    });
  }, [week]);

  // Compute the totals row (sum of all rows per day + grand total).
  const columnTotals = useMemo(() => {
    const totals = {};
    let grand = 0;
    DAYS.forEach((day) => {
      const t = rows.reduce(
        (sum, row) => sum + (Number(row.hours?.[day]) || 0),
        0,
      );
      totals[day] = t;
      grand += t;
    });
    totals._grand = grand;
    return totals;
  }, [rows]);

  return (
    <div className="w-full">
      {/* Scrollable table wrapper for small screens */}
      <div className="overflow-x-auto rounded-xl border border-slate-700">
        <table className="w-full min-w-[700px] border-collapse" role="grid">
          <thead>
            <tr className="bg-slate-800/80 border-b border-slate-700">
              <th className="text-left px-4 py-3 text-xs font-semibold text-slate-400 w-44">
                Description
              </th>
              {DAYS.map((day, i) => (
                <th key={day} className="px-2 py-3 text-center w-16">
                  <div className="text-xs font-semibold text-slate-300">
                    {DAY_LABELS[i]}
                  </div>
                  <div className="text-xs text-slate-500 font-normal">
                    {columnDates[i]?.toLocaleDateString("en", {
                      day: "numeric",
                      month: "short",
                    })}
                  </div>
                </th>
              ))}
              <th className="px-3 py-3 text-center w-16">
                <div className="text-xs font-semibold text-slate-400">
                  Total
                </div>
              </th>
              <th className="w-10" aria-label="Actions" />
            </tr>
          </thead>

          <tbody>
            {rows.map((row, rowIdx) => {
              const rowTotal = DAYS.reduce(
                (sum, day) => sum + (Number(row.hours?.[day]) || 0),
                0,
              );
              return (
                <React.Fragment key={row.id}>
                  <tr
                    className={[
                      "border-b border-slate-700/60",
                      "hover:bg-slate-800/40 transition-colors",
                    ].join(" ")}
                  >
                    {/* Description input */}
                    <td className="px-3 py-3">
                      <input
                        type="text"
                        value={row.label}
                        onChange={(e) => onLabelChange(row.id, e.target.value)}
                        placeholder={`Row ${rowIdx + 1}`}
                        className={[
                          "w-full px-3 py-2 rounded-lg text-sm",
                          "bg-slate-800 border border-slate-700",
                          "text-slate-200 placeholder-slate-600",
                          "focus:outline-none focus:border-amber-400/70 focus:ring-1 focus:ring-amber-400/40",
                          "transition-colors",
                        ].join(" ")}
                        aria-label={`Row ${rowIdx + 1} description`}
                      />
                    </td>

                    {/* Hour inputs */}
                    {DAYS.map((day) => (
                      <td key={day} className="px-1.5 py-3">
                        <input
                          type="number"
                          min="0"
                          max="24"
                          step="0.5"
                          value={row.hours?.[day] || ""}
                          onChange={(e) => {
                            const v = parseFloat(e.target.value);
                            onHoursChange(
                              row.id,
                              day,
                              isNaN(v) ? 0 : Math.max(0, Math.min(24, v)),
                            );
                          }}
                          className={[
                            "w-14 text-center px-2 py-2 rounded-lg text-sm",
                            "bg-slate-800 border",
                            row.hours?.[day] > 0
                              ? "border-amber-400/40 text-amber-200"
                              : "border-slate-700 text-slate-400",
                            "focus:outline-none focus:border-amber-400/70 focus:ring-1 focus:ring-amber-400/40",
                            "transition-colors appearance-none",
                          ].join(" ")}
                          aria-label={`${DAY_LABELS[DAYS.indexOf(day)]} hours for row ${rowIdx + 1}`}
                        />
                      </td>
                    ))}

                    {/* Row total */}
                    <td className="px-3 py-3 text-center">
                      <span
                        className={[
                          "text-sm font-semibold tabular-nums",
                          rowTotal > 0 ? "text-amber-300" : "text-slate-600",
                        ].join(" ")}
                      >
                        {rowTotal > 0 ? rowTotal.toFixed(1) : "—"}
                      </span>
                    </td>

                    {/* Delete row button */}
                    <td className="px-2 py-3">
                      <button
                        type="button"
                        onClick={() => onRemoveRow(row.id)}
                        disabled={rows.length <= 1}
                        className={[
                          "p-1.5 rounded-lg transition-colors",
                          "focus:outline-none focus-visible:ring-1 focus-visible:ring-amber-400",
                          rows.length <= 1
                            ? "text-slate-700 cursor-not-allowed"
                            : "text-slate-600 hover:text-red-400 hover:bg-red-400/10",
                        ].join(" ")}
                        aria-label="Remove row"
                        title={
                          rows.length <= 1
                            ? "Cannot remove the last row"
                            : "Remove row"
                        }
                      >
                        <svg
                          className="w-4 h-4"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={1.5}
                            d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                          />
                        </svg>
                      </button>
                    </td>
                  </tr>

                  {/* Row mapping panel */}
                  <tr className="border-b border-slate-700/40">
                    <td colSpan={DAYS.length + 3} className="px-3 pb-3">
                      <RowMapper
                        row={row}
                        systems={systems}
                        availableRows={availableRows}
                        onChange={(mappings) =>
                          onMappingsChange(row.id, mappings)
                        }
                      />
                    </td>
                  </tr>
                </React.Fragment>
              );
            })}
          </tbody>

          {/* Totals row */}
          <tfoot>
            <tr className="bg-slate-800/60 border-t-2 border-slate-600">
              <td className="px-4 py-3 text-xs font-semibold text-slate-400">
                Week Total
              </td>
              {DAYS.map((day) => (
                <td key={day} className="px-1.5 py-3 text-center">
                  <span
                    className={[
                      "text-xs font-semibold tabular-nums",
                      columnTotals[day] > 0
                        ? "text-slate-300"
                        : "text-slate-700",
                    ].join(" ")}
                  >
                    {columnTotals[day] > 0 ? columnTotals[day].toFixed(1) : "—"}
                  </span>
                </td>
              ))}
              <td className="px-3 py-3 text-center">
                <span
                  className={[
                    "text-sm font-bold tabular-nums",
                    columnTotals._grand > 0
                      ? "text-amber-300"
                      : "text-slate-700",
                  ].join(" ")}
                >
                  {columnTotals._grand > 0
                    ? columnTotals._grand.toFixed(1)
                    : "—"}
                </span>
              </td>
              <td />
            </tr>
          </tfoot>
        </table>
      </div>

      {/* Add row button */}
      <button
        type="button"
        onClick={onAddRow}
        className={[
          "mt-4 flex items-center gap-2 px-4 py-2.5 rounded-xl border border-dashed",
          "border-slate-600 text-slate-400 text-sm",
          "hover:border-amber-400/50 hover:text-amber-300 hover:bg-amber-400/5",
          "transition-all duration-200",
          "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
        ].join(" ")}
      >
        <svg
          className="w-4 h-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 4v16m8-8H4"
          />
        </svg>
        Add row
      </button>
    </div>
  );
}
