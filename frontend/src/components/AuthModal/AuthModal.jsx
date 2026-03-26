/**
 * AuthModal.jsx
 *
 * Renders the authentication modal for a time-reporting system.
 *
 * Two authentication modes are handled transparently:
 *
 * 1. Standard credential form
 *    The form fields come from system.authFields.  User fills them in and
 *    submits.  onSubmit is called with the field values.
 *
 * 2. OIDC / Azure popup flow (Maconomy with OpenID Connect)
 *    a. User fills in baseUrl + company and clicks "Connect".
 *    b. onSubmit is called → parent POSTs to backend → backend returns
 *       { status: "oidc_required", authUrl, redirectUri }.
 *    c. AuthModal receives the OIDC response via onOIDCRequired and opens
 *       a popup window pointing at the Azure login page.
 *    d. The popup loads the SPA at the redirect URI, detects ?code=, posts
 *       { type: "oidc_callback", code, state } to window.opener, then closes.
 *    e. AuthModal's message listener receives the code + state and calls
 *       onSubmit({ _oidcCode: code, _oidcState: state }) to complete the
 *       exchange.
 *
 * Security notes:
 * • The postMessage listener validates event.origin against window.location.origin
 *   so only messages from the same origin are accepted.
 * • The OIDC state nonce is verified server-side; the modal just forwards it.
 * • Credentials are never stored in component state beyond form submission.
 */

import React, { useState, useRef, useEffect, useCallback } from "react";

/**
 * AuthModal
 *
 * @param {Object}       props
 * @param {SystemInfo}   props.system          - The system being authenticated
 * @param {boolean}      props.isOpen          - Whether the modal is visible
 * @param {boolean}      props.isLoading       - Show loading state on submit
 * @param {string|null}  props.error           - Error from the last attempt
 * @param {Object|null}  props.oidcPending     - Set when OIDC popup is needed:
 *                                               { authUrl, redirectUri }
 * @param {function}     props.onSubmit        - Called with credential fields map
 * @param {function}     props.onClose         - Called to close the modal
 */
export default function AuthModal({
  system,
  isOpen,
  isLoading,
  error,
  oidcPending,
  onSubmit,
  onClose,
}) {
  const [fields, setFields] = useState({});
  const [oidcStatus, setOidcStatus] = useState(null); // 'waiting' | 'received' | null
  const firstInputRef = useRef(null);
  const popupRef = useRef(null);
  const pollRef = useRef(null);

  // Focus first input when modal opens.
  useEffect(() => {
    if (isOpen) {
      const t = setTimeout(() => firstInputRef.current?.focus(), 80);
      return () => clearTimeout(t);
    }
  }, [isOpen]);

  // Reset fields + OIDC state when modal opens for a different system.
  useEffect(() => {
    if (isOpen) {
      setFields({});
      setOidcStatus(null);
    }
  }, [isOpen, system?.id]);

  // ── OIDC popup management ────────────────────────────────────────────────

  // When the parent sets oidcPending, open the popup automatically.
  useEffect(() => {
    if (!oidcPending) {
      setOidcStatus(null);
      return;
    }

    openOIDCPopup(oidcPending.authUrl);
  }, [oidcPending]); // eslint-disable-line react-hooks/exhaustive-deps

  // Listen for the postMessage from the OIDC popup.
  const handlePopupMessage = useCallback(
    (event) => {
      // Only accept messages from our own origin.
      if (event.origin !== window.location.origin) return;
      if (!event.data || event.data.type !== "oidc_callback") return;

      const { code, state } = event.data;
      if (!code) return;

      setOidcStatus("received");
      clearPopupPoll();

      // Trigger the code-exchange POST.  The backend will verify the state
      // nonce and complete the token exchange.
      onSubmit({ _oidcCode: code, _oidcState: state || "" });
    },
    [onSubmit],
  );

  useEffect(() => {
    window.addEventListener("message", handlePopupMessage);
    return () => window.removeEventListener("message", handlePopupMessage);
  }, [handlePopupMessage]);

  function openOIDCPopup(authUrl) {
    // Close any stale popup from a previous attempt.
    if (popupRef.current && !popupRef.current.closed) {
      popupRef.current.close();
    }

    const width = 520;
    const height = 660;
    const left = Math.round(window.screenX + (window.outerWidth - width) / 2);
    const top = Math.round(window.screenY + (window.outerHeight - height) / 2);

    const popup = window.open(
      authUrl,
      "timesync_oidc",
      `width=${width},height=${height},left=${left},top=${top},` +
        "toolbar=no,menubar=no,scrollbars=yes,resizable=yes",
    );

    popupRef.current = popup;
    setOidcStatus("waiting");

    if (!popup) {
      // Popup was blocked by the browser.
      setOidcStatus(null);
      return;
    }

    // Poll for popup closure (user closed it without completing auth).
    pollRef.current = setInterval(() => {
      if (popup.closed) {
        clearPopupPoll();
        setOidcStatus((prev) => (prev === "waiting" ? null : prev));
      }
    }, 500);
  }

  function clearPopupPoll() {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }

  // Clean up on unmount.
  useEffect(
    () => () => {
      clearPopupPoll();
      if (popupRef.current && !popupRef.current.closed) {
        popupRef.current.close();
      }
    },
    [],
  );

  // ── Form helpers ──────────────────────────────────────────────────────────

  function handleChange(key, value) {
    setFields((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit(e) {
    e.preventDefault();
    onSubmit(fields);
  }

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget && !isLoading) onClose();
  }

  function handleKeyDown(e) {
    if (e.key === "Escape" && !isLoading) onClose();
  }

  if (!isOpen || !system) return null;

  // ── OIDC waiting state ────────────────────────────────────────────────────

  const showOIDCWaiting = oidcPending && oidcStatus === "waiting";
  const popupWasBlocked = oidcPending && oidcStatus === null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "rgba(0,0,0,0.7)", backdropFilter: "blur(4px)" }}
      onClick={handleBackdropClick}
      onKeyDown={handleKeyDown}
      role="dialog"
      aria-modal="true"
      aria-labelledby="auth-modal-title"
    >
      <div
        className="w-full max-w-md bg-slate-900 border border-slate-700 rounded-2xl
          shadow-2xl p-8"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h2
            id="auth-modal-title"
            className="text-lg font-bold text-slate-100"
          >
            Connect to {system.name}
          </h2>
          <button
            onClick={onClose}
            disabled={isLoading && !showOIDCWaiting}
            className="text-slate-400 hover:text-slate-200 transition-colors p-1 rounded-lg
              focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
            aria-label="Close"
          >
            <svg
              className="w-5 h-5"
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

        {/* Error message */}
        {error && (
          <div
            className="mb-5 p-4 rounded-lg bg-red-900/40 border border-red-700/60
            flex items-start gap-3"
            role="alert"
          >
            <svg
              className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5"
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
            <p className="text-red-300 text-sm">{error}</p>
          </div>
        )}

        {/* ── OIDC waiting state ─────────────────────────────────────────── */}
        {showOIDCWaiting ? (
          <div className="text-center py-4">
            <div
              className="w-16 h-16 rounded-full bg-amber-400/10 border-2 border-amber-400/30
              flex items-center justify-center mx-auto mb-5"
            >
              <svg
                className="w-7 h-7 text-amber-400 animate-pulse"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                />
              </svg>
            </div>
            <p className="text-slate-200 font-semibold mb-1">
              Waiting for Microsoft login…
            </p>
            <p className="text-slate-400 text-sm mb-6">
              Complete sign-in in the popup window that just opened.
              <br />
              Don't see it? Check for a blocked popup notification.
            </p>
            <button
              onClick={() => oidcPending && openOIDCPopup(oidcPending.authUrl)}
              className="text-amber-400 hover:text-amber-300 text-sm underline
                underline-offset-2 transition-colors"
            >
              Re-open popup
            </button>
          </div>
        ) : popupWasBlocked && oidcPending ? (
          /* ── Popup blocked fallback ──────────────────────────────────── */
          <div className="text-center py-4">
            <div
              className="w-16 h-16 rounded-full bg-red-400/10 border-2 border-red-400/30
              flex items-center justify-center mx-auto mb-5"
            >
              <svg
                className="w-7 h-7 text-red-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"
                />
              </svg>
            </div>
            <p className="text-slate-200 font-semibold mb-2">
              Popup was blocked
            </p>
            <p className="text-slate-400 text-sm mb-6">
              Allow popups for this site and try again, or open the login page
              directly.
            </p>
            <div className="flex flex-col gap-2">
              <button
                onClick={() => openOIDCPopup(oidcPending.authUrl)}
                className="w-full py-3 rounded-xl bg-amber-400 hover:bg-amber-300
                  text-slate-900 font-semibold text-sm transition-all"
              >
                Try popup again
              </button>
              <a
                href={oidcPending.authUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="w-full py-3 rounded-xl border border-slate-600 text-slate-300
                  hover:border-slate-400 text-sm font-medium text-center transition-all"
              >
                Open in new tab instead ↗
              </a>
            </div>
          </div>
        ) : (
          /* ── Standard credential form ──────────────────────────────────── */
          <form onSubmit={handleSubmit} noValidate className="space-y-4">
            {system.authFields.map((field, idx) => (
              <div key={field.key}>
                <label
                  htmlFor={`auth-${field.key}`}
                  className="block text-sm font-medium text-slate-300 mb-1.5"
                >
                  {field.label}
                  {field.required && (
                    <span className="text-amber-400 ml-1" aria-hidden="true">
                      *
                    </span>
                  )}
                </label>
                <input
                  ref={idx === 0 ? firstInputRef : undefined}
                  id={`auth-${field.key}`}
                  type={
                    field.type === "password"
                      ? "password"
                      : field.type === "url"
                        ? "url"
                        : "text"
                  }
                  autoComplete={
                    field.type === "password" ? "current-password" : "off"
                  }
                  placeholder={field.placeholder || ""}
                  required={field.required}
                  disabled={isLoading}
                  value={fields[field.key] || ""}
                  onChange={(e) => handleChange(field.key, e.target.value)}
                  className={[
                    "w-full px-4 py-3 rounded-xl text-sm",
                    "bg-slate-800 border border-slate-600",
                    "text-slate-100 placeholder-slate-500",
                    "focus:outline-none focus:border-amber-400 focus:ring-1 focus:ring-amber-400",
                    "disabled:opacity-50 disabled:cursor-not-allowed transition-colors",
                  ].join(" ")}
                />
                {field.helpText && (
                  <p className="mt-1.5 text-xs text-slate-500">
                    {field.helpText}
                  </p>
                )}
              </div>
            ))}

            <button
              type="submit"
              disabled={isLoading}
              className={[
                "w-full mt-2 py-3 px-6 rounded-xl font-semibold text-sm",
                "transition-all duration-200",
                "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400",
                isLoading
                  ? "bg-slate-700 text-slate-400 cursor-not-allowed"
                  : "bg-amber-400 hover:bg-amber-300 text-slate-900 shadow-lg shadow-amber-400/20",
              ].join(" ")}
            >
              {isLoading ? (
                <span className="flex items-center justify-center gap-2">
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
                  {/* Show different label while discovering OIDC config */}
                  Connecting…
                </span>
              ) : (
                `Connect to ${system.name}`
              )}
            </button>

            <p className="text-center text-xs text-slate-500">
              Your credentials are sent securely and never stored in the
              browser.
            </p>
          </form>
        )}
      </div>
    </div>
  );
}
