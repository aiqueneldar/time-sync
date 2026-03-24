/**
 * AuthModal.jsx
 *
 * Renders a login form for a time-reporting system.
 *
 * The form fields are driven entirely by the system's `authFields` array
 * returned from the backend.  Adding a new auth field to an adapter
 * automatically surfaces it here without any frontend code changes.
 *
 * Security:
 * • Credentials are sent directly to POST /api/auth/{systemId} in the
 *   request body; they are never stored in component state beyond the
 *   form submission lifecycle.
 * • Password fields are rendered as type="password" so the browser
 *   does not display or auto-save them in an insecure context.
 */

import React, { useState, useRef, useEffect } from "react";

/**
 * AuthModal
 *
 * @param {Object}       props
 * @param {SystemInfo}   props.system       - The system being authenticated
 * @param {boolean}      props.isOpen       - Whether the modal is visible
 * @param {boolean}      props.isLoading    - Show a loading state on submit
 * @param {string|null}  props.error        - Error message from the last attempt
 * @param {function}     props.onSubmit     - Called with credential fields map
 * @param {function}     props.onClose      - Called to close the modal
 */
export default function AuthModal({
  system,
  isOpen,
  isLoading,
  error,
  onSubmit,
  onClose,
}) {
  const [fields, setFields] = useState({});
  const firstInputRef = useRef(null);

  // Focus the first input when the modal opens.
  useEffect(() => {
    if (isOpen) {
      // Give the modal animation time to complete before focusing.
      const t = setTimeout(() => firstInputRef.current?.focus(), 80);
      return () => clearTimeout(t);
    }
  }, [isOpen]);

  // Reset fields when the modal is opened for a different system.
  useEffect(() => {
    if (isOpen) setFields({});
  }, [isOpen, system?.id]);

  if (!isOpen || !system) return null;

  function handleChange(key, value) {
    setFields((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit(e) {
    e.preventDefault();
    onSubmit(fields);
  }

  // Close on backdrop click.
  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) onClose();
  }

  // Close on Escape key.
  function handleKeyDown(e) {
    if (e.key === "Escape") onClose();
  }

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
        className="w-full max-w-md bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl p-8"
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
            disabled={isLoading}
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

        {/* Form - fields rendered from backend metadata */}
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
                  "disabled:opacity-50 disabled:cursor-not-allowed",
                  "transition-colors",
                ].join(" ")}
              />
              {field.helpText && (
                <p className="mt-1.5 text-xs text-slate-500">
                  {field.helpText}
                </p>
              )}
            </div>
          ))}

          {/* Submit */}
          <button
            type="submit"
            disabled={isLoading}
            className={[
              "w-full mt-2 py-3 px-6 rounded-xl font-semibold text-sm",
              "transition-all duration-200",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-900",
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
                Connecting…
              </span>
            ) : (
              `Connect to ${system.name}`
            )}
          </button>
        </form>

        <p className="mt-4 text-center text-xs text-slate-500">
          Your credentials are sent securely to the backend and are never stored
          in your browser.
        </p>
      </div>
    </div>
  );
}
