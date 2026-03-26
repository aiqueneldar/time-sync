/**
 * SetupPage.jsx
 *
 * The initial setup flow shown when the user first opens TimeSync.
 * Two steps:
 *   1. Select which time-reporting systems to use.
 *   2. Authenticate with each selected system.
 *
 * Once all selected systems are authenticated the user proceeds to the
 * timesheet entry page.
 */

import React, { useEffect, useState } from "react";
import SystemSelector from "../components/SystemSelector/SystemSelector";
import AuthModal from "../components/AuthModal/AuthModal";
import { fetchSystems, authenticate, getAuthStatus } from "../services/api";
import { useApp, ACTIONS } from "../context/AppContext";

export default function SetupPage() {
  const { state, dispatch, setLoading, setError, clearError } = useApp();
  const { systems, selectedSystems, authStatuses } = state;

  const [authModalSystem, setAuthModalSystem] = useState(null); // SystemInfo | null
  const [oidcPending, setOidcPending] = useState(null); // { authUrl, redirectUri } | null
  const [step, setStep] = useState("select"); // 'select' | 'auth'

  // ── Load available systems from backend ──────────────────────────────

  useEffect(() => {
    async function load() {
      setLoading("systems", true);
      clearError("systems");
      try {
        const data = await fetchSystems();
        dispatch({ type: ACTIONS.SET_SYSTEMS, payload: data.systems || [] });
      } catch (err) {
        setError("systems", err.message || "Failed to load systems");
      } finally {
        setLoading("systems", false);
      }
    }
    load();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Sync auth statuses when the step changes to 'auth' ───────────────

  useEffect(() => {
    if (step !== "auth") return;
    selectedSystems.forEach(async (id) => {
      try {
        const status = await getAuthStatus(id);
        dispatch({ type: ACTIONS.SET_AUTH_STATUS, payload: status });
      } catch (_) {
        /* not yet authenticated, that's fine */
      }
    });
  }, [step]); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Handlers ──────────────────────────────────────────────────────────

  function toggleSystem(id) {
    const next = selectedSystems.includes(id)
      ? selectedSystems.filter((s) => s !== id)
      : [...selectedSystems, id];
    dispatch({ type: ACTIONS.SET_SELECTED_SYSTEMS, payload: next });
  }

  function handleProceedToAuth() {
    if (selectedSystems.length === 0) return;
    setStep("auth");
  }

  function handleBack() {
    setStep("select");
  }

  function openAuthModal(systemId) {
    const system = systems.find((s) => s.id === systemId);
    if (!system) return;
    clearError(`auth_${systemId}`);
    setAuthModalSystem(system);
  }

  async function handleAuthSubmit(fields) {
    if (!authModalSystem) return;
    const id = authModalSystem.id;

    setLoading(`auth_${id}`, true);
    clearError(`auth_${id}`);
    try {
      const result = await authenticate(id, fields);

      // Backend returned 202: OIDC popup required.
      // Store the auth URL and redirect URI so AuthModal can open the popup.
      if (result?.status === "oidc_required") {
        setOidcPending({
          authUrl: result.authUrl,
          redirectUri: result.redirectUri,
        });
        // Keep the modal open and loading state off while waiting for popup.
        setLoading(`auth_${id}`, false);
        return;
      }

      // Normal success (200): auth complete.
      dispatch({ type: ACTIONS.SET_AUTH_STATUS, payload: result });
      setAuthModalSystem(null);
      setOidcPending(null);
    } catch (err) {
      setOidcPending(null);
      setError(`auth_${id}`, err.message || "Authentication failed");
    } finally {
      setLoading(`auth_${id}`, false);
    }
  }

  function handleProceedToTimesheet() {
    dispatch({ type: ACTIONS.SET_PAGE, payload: "timesheet" });
  }

  // ── Derived state ─────────────────────────────────────────────────────

  const isLoadingSystems = state.loadingStates["systems"];
  const systemsError = state.errors["systems"];
  const selectedSystemInfos = systems.filter((s) =>
    selectedSystems.includes(s.id),
  );
  const allAuthenticated =
    selectedSystems.length > 0 &&
    selectedSystems.every((id) => authStatuses[id]?.authenticated);

  const modalIsLoading = authModalSystem
    ? !!state.loadingStates[`auth_${authModalSystem.id}`]
    : false;
  const modalError = authModalSystem
    ? state.errors[`auth_${authModalSystem.id}`] || null
    : null;

  // ── Render ────────────────────────────────────────────────────────────

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center p-6">
      {/* Background grid decoration */}
      <div
        className="fixed inset-0 pointer-events-none opacity-30"
        style={{
          backgroundImage:
            "radial-gradient(circle at 1px 1px, rgba(148,163,184,0.15) 1px, transparent 0)",
          backgroundSize: "32px 32px",
        }}
        aria-hidden="true"
      />

      <div className="relative w-full max-w-xl">
        {/* Logo / wordmark */}
        <div className="mb-10 text-center">
          <div className="inline-flex items-center gap-3 mb-3">
            <span className="text-3xl" aria-hidden="true">
              ⏱
            </span>
            <h1 className="text-3xl font-black text-slate-100 tracking-tight">
              Time<span className="text-amber-400">Sync</span>
            </h1>
          </div>
          <p className="text-slate-400 text-sm">
            Report your hours once. Sync everywhere.
          </p>
        </div>

        {/* Card */}
        <div
          className="bg-slate-900/80 backdrop-blur border border-slate-700/80
          rounded-2xl shadow-2xl p-8"
        >
          {step === "select" ? (
            <>
              {/* Step header */}
              <div className="flex items-center gap-3 mb-6">
                <span
                  className="w-7 h-7 rounded-full bg-amber-400 text-slate-900
                  text-xs font-bold flex items-center justify-center"
                >
                  1
                </span>
                <div>
                  <h2 className="text-base font-bold text-slate-100">
                    Choose your time-reporting systems
                  </h2>
                  <p className="text-xs text-slate-400 mt-0.5">
                    Select all the systems you want to sync time into.
                  </p>
                </div>
              </div>

              {/* System selector */}
              {isLoadingSystems ? (
                <div className="flex items-center justify-center py-12">
                  <svg
                    className="w-6 h-6 animate-spin text-amber-400"
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
                </div>
              ) : systemsError ? (
                <div className="py-8 text-center">
                  <p className="text-red-400 text-sm mb-4">{systemsError}</p>
                  <button
                    onClick={() => window.location.reload()}
                    className="text-xs text-slate-400 underline hover:text-slate-200"
                  >
                    Retry
                  </button>
                </div>
              ) : (
                <SystemSelector
                  systems={systems}
                  selectedIds={selectedSystems}
                  onToggle={toggleSystem}
                />
              )}

              {/* Continue button */}
              <button
                onClick={handleProceedToAuth}
                disabled={selectedSystems.length === 0}
                className={[
                  "w-full mt-6 py-3 px-6 rounded-xl font-semibold text-sm",
                  "transition-all duration-200",
                  "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
                  selectedSystems.length === 0
                    ? "bg-slate-800 text-slate-600 cursor-not-allowed"
                    : "bg-amber-400 hover:bg-amber-300 text-slate-900 shadow-lg shadow-amber-400/20",
                ].join(" ")}
              >
                Continue →
              </button>
            </>
          ) : (
            <>
              {/* Step 2 – Authenticate */}
              <div className="flex items-center gap-3 mb-6">
                <span
                  className="w-7 h-7 rounded-full bg-amber-400 text-slate-900
                  text-xs font-bold flex items-center justify-center"
                >
                  2
                </span>
                <div>
                  <h2 className="text-base font-bold text-slate-100">
                    Connect to your systems
                  </h2>
                  <p className="text-xs text-slate-400 mt-0.5">
                    Click each system to enter your credentials.
                  </p>
                </div>
              </div>

              {/* Auth status cards */}
              <div className="space-y-3">
                {selectedSystemInfos.map((system) => {
                  const status = authStatuses[system.id];
                  const authed = status?.authenticated;
                  return (
                    <div
                      key={system.id}
                      className={[
                        "flex items-center gap-4 p-4 rounded-xl border",
                        "transition-all duration-200",
                        authed
                          ? "border-emerald-500/40 bg-emerald-500/5"
                          : "border-slate-700 bg-slate-800/50",
                      ].join(" ")}
                    >
                      <div className="flex-1">
                        <p className="text-sm font-semibold text-slate-200">
                          {system.name}
                        </p>
                        <p
                          className={`text-xs mt-0.5 ${authed ? "text-emerald-400" : "text-slate-500"}`}
                        >
                          {authed ? "✓ Connected" : "Not connected"}
                        </p>
                      </div>
                      <button
                        onClick={() => openAuthModal(system.id)}
                        className={[
                          "px-4 py-2 rounded-lg text-xs font-semibold transition-all",
                          "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
                          authed
                            ? "bg-slate-700 text-slate-300 hover:bg-slate-600"
                            : "bg-amber-400 text-slate-900 hover:bg-amber-300 shadow-md shadow-amber-400/20",
                        ].join(" ")}
                      >
                        {authed ? "Reconnect" : "Connect"}
                      </button>
                    </div>
                  );
                })}
              </div>

              {/* Navigation */}
              <div className="flex gap-3 mt-6">
                <button
                  onClick={handleBack}
                  className="flex-1 py-3 px-4 rounded-xl font-semibold text-sm
                    border border-slate-700 text-slate-400
                    hover:border-slate-500 hover:text-slate-300
                    transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
                >
                  ← Back
                </button>
                <button
                  onClick={handleProceedToTimesheet}
                  disabled={!allAuthenticated}
                  className={[
                    "flex-[2] py-3 px-6 rounded-xl font-semibold text-sm",
                    "transition-all duration-200",
                    "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
                    !allAuthenticated
                      ? "bg-slate-800 text-slate-600 cursor-not-allowed"
                      : "bg-amber-400 hover:bg-amber-300 text-slate-900 shadow-lg shadow-amber-400/20",
                  ].join(" ")}
                >
                  {allAuthenticated
                    ? "Start reporting time →"
                    : `Connect ${selectedSystems.filter((id) => !authStatuses[id]?.authenticated).length} remaining system(s)`}
                </button>
              </div>
            </>
          )}
        </div>

        {/* Step indicator */}
        <div className="flex justify-center gap-2 mt-6" aria-hidden="true">
          {["select", "auth"].map((s, i) => (
            <div
              key={s}
              className={[
                "w-2 h-2 rounded-full transition-all",
                step === s ? "bg-amber-400 w-6" : "bg-slate-700",
              ].join(" ")}
            />
          ))}
        </div>
      </div>

      {/* Auth modal */}
      <AuthModal
        system={authModalSystem}
        isOpen={!!authModalSystem}
        isLoading={modalIsLoading}
        error={modalError}
        oidcPending={oidcPending}
        onSubmit={handleAuthSubmit}
        onClose={() => {
          setAuthModalSystem(null);
          setOidcPending(null);
        }}
      />
    </div>
  );
}
