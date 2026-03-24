/**
 * TimesheetPage.jsx
 *
 * The main application page shown after setup.
 *
 * Features:
 * ─────────────────────────────────────────────────────────────────────────
 * • Week navigator (← this week →)
 * • Time-entry table with dynamic rows and backend row mapping
 * • Sync button that dispatches to the async backend and tracks progress
 * • Real-time status chips updated via Server-Sent Events
 * • Users can continue editing while a sync is in progress
 */

import React, { useEffect, useRef, useCallback } from "react";
import TimeTable from "../components/TimeTable/TimeTable";
import SyncStatusBar from "../components/SyncStatus/SyncStatus";
import AuthModal from "../components/AuthModal/AuthModal";
import {
  fetchAvailableRows,
  syncTimesheet,
  openSyncStatusStream,
  authenticate,
} from "../services/api";
import { useApp, ACTIONS } from "../context/AppContext";

export default function TimesheetPage() {
  const { state, dispatch, setLoading, setError, clearError } = useApp();
  const {
    systems,
    selectedSystems,
    authStatuses,
    week,
    rows,
    availableRows,
    syncStatuses,
    isSyncing,
  } = state;

  const sseRef = useRef(null); // EventSource reference
  const [authModalSystem, setAuthModalSystem] = React.useState(null);

  // ── Derived values ────────────────────────────────────────────────────

  const selectedSystemInfos = systems.filter((s) =>
    selectedSystems.includes(s.id),
  );
  const canSync = rows.some((r) => r.mappings?.length > 0);

  // ── Fetch available rows for each authenticated system ─────────────────

  useEffect(() => {
    selectedSystems.forEach(async (id) => {
      if (!authStatuses[id]?.authenticated) return;
      try {
        const data = await fetchAvailableRows(id);
        dispatch({
          type: ACTIONS.SET_AVAILABLE_ROWS,
          payload: { systemId: id, rows: data.rows },
        });
      } catch (err) {
        // Non-fatal: the user can still enter time, just can't map to rows.
        console.warn(`Could not fetch rows for ${id}:`, err.message);
      }
    });
  }, [selectedSystems, authStatuses]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── SSE sync-status stream ─────────────────────────────────────────────

  const handleSyncStatus = useCallback(
    (status) => {
      dispatch({ type: ACTIONS.SET_SYNC_STATUS, payload: status });

      // Update isSyncing based on whether any system is still syncing.
      // Check on next tick so the state has the latest status applied.
      setTimeout(() => {
        dispatch((getState) => {
          // Note: useReducer doesn't support thunks natively; we use
          // a separate SET_IS_SYNCING action instead (see below).
        });
      }, 0);
    },
    [dispatch],
  );

  useEffect(() => {
    // Open SSE stream.
    const es = openSyncStatusStream(
      (status) => {
        dispatch({ type: ACTIONS.SET_SYNC_STATUS, payload: status });
      },
      (err) => {
        // SSE errors are non-fatal; the browser will auto-reconnect.
        console.warn("SSE connection error", err);
      },
    );
    sseRef.current = es;

    return () => {
      es.close();
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Derive isSyncing from syncStatuses whenever they change.
  useEffect(() => {
    const syncing = Object.values(syncStatuses).some(
      (s) => s?.state === "syncing",
    );
    dispatch({ type: ACTIONS.SET_IS_SYNCING, payload: syncing });
  }, [syncStatuses, dispatch]);

  // ── Week navigation ───────────────────────────────────────────────────

  function offsetWeek(delta) {
    // Convert ISO year/week to a Date, add delta weeks, convert back.
    const monday = weekStart(week.year, week.week);
    monday.setDate(monday.getDate() + delta * 7);
    const [newYear, newWeek] = dateToISOWeek(monday);
    dispatch({
      type: ACTIONS.SET_WEEK,
      payload: { year: newYear, week: newWeek },
    });
  }

  // ── Time-entry handlers ───────────────────────────────────────────────

  const handleLabelChange = (rowId, label) =>
    dispatch({ type: ACTIONS.UPDATE_ROW_LABEL, payload: { id: rowId, label } });

  const handleHoursChange = (rowId, day, value) =>
    dispatch({
      type: ACTIONS.UPDATE_ROW_HOURS,
      payload: { id: rowId, day, value },
    });

  const handleMappingsChange = (rowId, mappings) =>
    dispatch({
      type: ACTIONS.UPDATE_ROW_MAPPINGS,
      payload: { id: rowId, mappings },
    });

  const handleAddRow = () => dispatch({ type: ACTIONS.ADD_ROW });
  const handleRemoveRow = (rowId) =>
    dispatch({ type: ACTIONS.REMOVE_ROW, payload: rowId });

  // ── Sync ──────────────────────────────────────────────────────────────

  async function handleSync() {
    clearError("sync");
    try {
      await syncTimesheet({ week, rows });
      // Status updates will arrive via SSE.
    } catch (err) {
      setError("sync", err.message || "Sync failed");
    }
  }

  // ── Auth reconnect ────────────────────────────────────────────────────

  async function handleAuthSubmit(fields) {
    if (!authModalSystem) return;
    const id = authModalSystem.id;
    setLoading(`auth_${id}`, true);
    clearError(`auth_${id}`);
    try {
      const status = await authenticate(id, fields);
      dispatch({ type: ACTIONS.SET_AUTH_STATUS, payload: status });
      setAuthModalSystem(null);
    } catch (err) {
      setError(`auth_${id}`, err.message || "Authentication failed");
    } finally {
      setLoading(`auth_${id}`, false);
    }
  }

  // ── Render ────────────────────────────────────────────────────────────

  const weekLabel = formatWeekLabel(week);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      {/* Top bar */}
      <header
        className="sticky top-0 z-40 border-b border-slate-800
        bg-slate-950/90 backdrop-blur px-6 py-4"
      >
        <div className="max-w-7xl mx-auto flex items-center justify-between gap-4 flex-wrap">
          {/* Logo */}
          <div className="flex items-center gap-2.5 flex-shrink-0">
            <span className="text-xl" aria-hidden="true">
              ⏱
            </span>
            <span className="text-lg font-black tracking-tight">
              Time<span className="text-amber-400">Sync</span>
            </span>
          </div>

          {/* Week navigator */}
          <div className="flex items-center gap-3">
            <button
              onClick={() => offsetWeek(-1)}
              className="p-2 rounded-lg text-slate-400 hover:text-slate-200
                hover:bg-slate-800 transition-all
                focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
              aria-label="Previous week"
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
                  d="M15 19l-7-7 7-7"
                />
              </svg>
            </button>
            <span className="text-sm font-semibold text-slate-200 min-w-[14rem] text-center tabular-nums">
              {weekLabel}
            </span>
            <button
              onClick={() => offsetWeek(1)}
              className="p-2 rounded-lg text-slate-400 hover:text-slate-200
                hover:bg-slate-800 transition-all
                focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
              aria-label="Next week"
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
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>

          {/* Sync button + status chips */}
          <div className="flex items-center gap-4 flex-wrap">
            <SyncStatusBar
              systems={selectedSystemInfos}
              syncStatuses={syncStatuses}
            />

            <button
              onClick={handleSync}
              disabled={!canSync}
              className={[
                "flex items-center gap-2.5 px-5 py-2.5 rounded-xl font-bold text-sm",
                "transition-all duration-200 shadow-lg",
                "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
                !canSync
                  ? "bg-slate-800 text-slate-600 cursor-not-allowed shadow-none"
                  : isSyncing
                    ? "bg-amber-500/20 border border-amber-400/50 text-amber-300 shadow-amber-400/10 cursor-wait"
                    : "bg-amber-400 hover:bg-amber-300 text-slate-900 shadow-amber-400/30",
              ].join(" ")}
              title={
                !canSync
                  ? "Map at least one row to a system before syncing"
                  : undefined
              }
            >
              {isSyncing ? (
                <svg
                  className="w-4 h-4 animate-spin"
                  viewBox="0 0 24 24"
                  fill="none"
                >
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                  />
                </svg>
              ) : (
                <svg
                  className="w-4 h-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2.5}
                    d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                  />
                </svg>
              )}
              {isSyncing ? "Syncing…" : "Sync"}
            </button>
          </div>
        </div>
      </header>

      {/* Error banner */}
      {state.errors["sync"] && (
        <div className="max-w-7xl mx-auto px-6 pt-4">
          <div
            className="flex items-center gap-3 p-4 rounded-xl bg-red-900/30
            border border-red-700/50"
            role="alert"
          >
            <svg
              className="w-5 h-5 text-red-400 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p className="text-red-300 text-sm flex-1">
              {state.errors["sync"]}
            </p>
            <button
              onClick={() => clearError("sync")}
              className="text-red-500
              hover:text-red-300 transition-colors"
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
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        </div>
      )}

      {/* Connected systems pills */}
      <div className="max-w-7xl mx-auto px-6 pt-6 pb-2 flex items-center gap-3 flex-wrap">
        <span className="text-xs text-slate-500 font-medium uppercase tracking-wider">
          Connected:
        </span>
        {selectedSystemInfos.map((system) => {
          const authed = authStatuses[system.id]?.authenticated;
          return (
            <button
              key={system.id}
              onClick={() => !authed && setAuthModalSystem(system)}
              title={
                authed
                  ? "Click to reconnect"
                  : "Not connected – click to authenticate"
              }
              className={[
                "inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium",
                "border transition-all",
                authed
                  ? "border-emerald-700/50 bg-emerald-900/20 text-emerald-400"
                  : "border-red-700/50 bg-red-900/20 text-red-400 cursor-pointer hover:border-red-500",
              ].join(" ")}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${authed ? "bg-emerald-400" : "bg-red-400"}`}
                aria-hidden="true"
              />
              {system.name}
            </button>
          );
        })}
        <button
          onClick={() => dispatch({ type: ACTIONS.SET_PAGE, payload: "setup" })}
          className="ml-auto text-xs text-slate-500 hover:text-slate-300 transition-colors
            underline underline-offset-2"
        >
          Manage systems
        </button>
      </div>

      {/* Main content */}
      <main className="max-w-7xl mx-auto px-6 pb-12 pt-4">
        {/* Hint text if no mappings */}
        {!canSync && (
          <div
            className="mb-4 flex items-start gap-3 p-4 rounded-xl
            bg-slate-800/50 border border-slate-700/60"
          >
            <svg
              className="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <p className="text-sm text-slate-300">
              Enter your hours, then use{" "}
              <strong className="text-amber-300">Map to system rows</strong>{" "}
              below each row to choose where to sync them. Press{" "}
              <strong className="text-amber-300">Sync</strong> when done.
            </p>
          </div>
        )}

        {/* Time table */}
        <TimeTable
          rows={rows}
          week={week}
          systems={selectedSystemInfos}
          availableRows={availableRows}
          onLabelChange={handleLabelChange}
          onHoursChange={handleHoursChange}
          onMappingsChange={handleMappingsChange}
          onAddRow={handleAddRow}
          onRemoveRow={handleRemoveRow}
        />
      </main>

      {/* Auth reconnect modal */}
      <AuthModal
        system={authModalSystem}
        isOpen={!!authModalSystem}
        isLoading={
          authModalSystem
            ? !!state.loadingStates[`auth_${authModalSystem.id}`]
            : false
        }
        error={
          authModalSystem
            ? state.errors[`auth_${authModalSystem.id}`] || null
            : null
        }
        onSubmit={handleAuthSubmit}
        onClose={() => setAuthModalSystem(null)}
      />
    </div>
  );
}

// ─── Week utilities ────────────────────────────────────────────────────────

function weekStart(year, week) {
  const jan4 = new Date(year, 0, 4);
  const day = jan4.getDay() || 7;
  const mon1 = new Date(jan4);
  mon1.setDate(jan4.getDate() - (day - 1));
  const result = new Date(mon1);
  result.setDate(mon1.getDate() + (week - 1) * 7);
  return result;
}

function dateToISOWeek(d) {
  const date = new Date(d);
  date.setHours(0, 0, 0, 0);
  date.setDate(date.getDate() + 3 - ((date.getDay() + 6) % 7));
  const week1 = new Date(date.getFullYear(), 0, 4);
  const weekNum =
    1 +
    Math.round(
      ((date - week1) / 86400000 - 3 + ((week1.getDay() + 6) % 7)) / 7,
    );
  return [date.getFullYear(), weekNum];
}

function formatWeekLabel({ year, week }) {
  const monday = weekStart(year, week);
  const sunday = new Date(monday);
  sunday.setDate(monday.getDate() + 6);

  const fmt = (d) =>
    d.toLocaleDateString("en", { day: "numeric", month: "short" });
  return `Week ${week} · ${fmt(monday)} – ${fmt(sunday)} ${year}`;
}
