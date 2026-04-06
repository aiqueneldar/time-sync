/**
 * App.jsx
 *
 * Root application component.
 *
 * OIDC popup detection
 * ────────────────────
 * When Azure redirects back to the app after authentication it lands on
 * the registered redirectUri (e.g. http://localhost/).  If the app is
 * running inside a popup window it means this IS the OIDC callback page.
 *
 * We detect this by checking:
 *   1. window.opener is not null  →  we are a popup
 *   2. ?code= is present in the URL  →  this is an auth callback
 *
 * If both are true we post the code + state to the parent window and close
 * ourselves.  The parent's AuthModal is listening for this message.
 */

import React, { useEffect, useState } from "react";
import { AppProvider, useApp } from "./context/AppContext";
import SetupPage from "./pages/SetupPage";
import TimesheetPage from "./pages/TimesheetPage";

// ─── OIDC popup handler ────────────────────────────────────────────────────

/**
 * OIDCCallbackHandler renders a minimal "Completing login…" screen when the
 * app detects it is running inside the OIDC redirect popup.  It posts the
 * authorization code + state to the parent window and closes the popup.
 */
function OIDCCallbackHandler({ code, state }) {
  useEffect(() => {
    if (!window.opener) return;

    // Post the code and state to the parent SPA.
    // We use '*' as the target origin here because the redirect URI configured
    // in Azure may be a bare origin (e.g. http://localhost/) which matches the
    // parent.  In production, replace '*' with your exact origin for safety.
    window.opener.postMessage(
      { type: "oidc_callback", code, state },
      window.location.origin,
    );

    // Give the parent a moment to receive the message before closing.
    const t = setTimeout(() => window.close(), 300);
    return () => clearTimeout(t);
  }, [code, state]);

  return (
    <div className="min-h-screen bg-slate-950 flex items-center justify-center">
      <div className="text-center">
        <svg
          className="w-8 h-8 animate-spin text-amber-400 mx-auto mb-4"
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
        <p className="text-slate-300 text-sm font-medium">Completing login…</p>
        <p className="text-slate-500 text-xs mt-1">
          This window will close automatically.
        </p>
      </div>
    </div>
  );
}

// ─── Main app ──────────────────────────────────────────────────────────────

function AppContent() {
  const { state } = useApp();
  return state.page === "timesheet" ? <TimesheetPage /> : <SetupPage />;
}

export default function App() {
  // Detect OIDC popup callback before rendering the full app.
  const [oidcCallback, setOidcCallback] = useState(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");

    // We're in the OIDC popup if: we have a code param AND a parent window.
    if (code && window.opener) {
      setOidcCallback({ code, state: state || "" });
    }
  }, []);

  if (oidcCallback) {
    return (
      <OIDCCallbackHandler
        code={oidcCallback.code}
        state={oidcCallback.state}
      />
    );
  }

  return (
    <AppProvider>
      <AppContent />
    </AppProvider>
  );
}
