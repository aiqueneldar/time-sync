/**
 * api.js
 *
 * Centralised API client for the TimeSync backend.
 *
 * Every request automatically attaches the X-Session-ID header and handles
 * common error patterns (network failures, 4xx/5xx responses).
 *
 * No API tokens or credentials are stored here; they are sent once to the
 * backend during authentication and are handled entirely server-side.
 */

import { getSessionID } from "./tokenStore";

// Base URL is injected at build time via Vite's import.meta.env.
// Falls back to same-origin so Docker / reverse-proxy deployments work without config.
const BASE_URL = import.meta.env.VITE_API_BASE_URL || "";

/**
 * ApiError wraps non-2xx HTTP responses with the server's error message.
 */
export class ApiError extends Error {
  /** @param {number} status @param {string} message */
  constructor(status, message) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

/**
 * request is the core fetch wrapper.
 *
 * @param {string} path   - API path, e.g. '/api/systems'
 * @param {RequestInit} [options]
 * @returns {Promise<any>} Parsed JSON response body
 * @throws {ApiError} on non-2xx responses (except 202 which is returned normally)
 */
async function request(path, options = {}) {
  const headers = {
    Accept: "application/json",
    "X-Session-ID": getSessionID(),
    ...(options.headers || {}),
  };

  if (options.body && !options.headers?.["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }

  const response = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
    // Never send cookies – authentication is header-based.
    credentials: "omit",
  });

  // 202 Accepted is used for the OIDC flow: return the body normally.
  if (response.status === 202) {
    return response.json();
  }

  if (!response.ok) {
    let message = `HTTP ${response.status}`;
    try {
      const errBody = await response.json();
      message = errBody.error || errBody.message || message;
    } catch (_) {
      message = response.statusText || message;
    }
    throw new ApiError(response.status, message);
  }

  const contentType = response.headers.get("Content-Type") || "";
  if (contentType.includes("application/json")) {
    return response.json();
  }
  return null;
}

// ─── Systems ──────────────────────────────────────────────────────────────

/**
 * fetchSystems returns the list of available time-reporting systems.
 * @returns {Promise<{systems: SystemInfo[]}>}
 */
export async function fetchSystems() {
  return request("/api/systems");
}

// ─── Authentication ───────────────────────────────────────────────────────

/**
 * authenticate submits credentials for a given system.
 * The backend exchanges them for a token which is stored server-side only.
 *
 * @param {string} systemId
 * @param {Record<string,string>} fields  - Key/value pairs matching AuthField.key
 * @returns {Promise<AuthStatus>}
 */
export async function authenticate(systemId, fields) {
  return request(`/api/auth/${systemId}`, {
    method: "POST",
    body: JSON.stringify(fields),
  });
}

/**
 * getAuthStatus checks whether the session is authenticated with a system.
 * @param {string} systemId
 * @returns {Promise<AuthStatus>}
 */
export async function getAuthStatus(systemId) {
  return request(`/api/auth/${systemId}`);
}

/**
 * logout removes the token for a system from the server-side session.
 * @param {string} systemId
 * @returns {Promise<void>}
 */
export async function logout(systemId) {
  return request(`/api/auth/${systemId}`, { method: "DELETE" });
}

// ─── Timesheet rows ───────────────────────────────────────────────────────

/**
 * fetchAvailableRows fetches the bookable rows from a given system.
 * @param {string} systemId
 * @returns {Promise<{systemId: string, rows: SystemRow[]}>}
 */
export async function fetchAvailableRows(systemId) {
  return request(`/api/timesheets/${systemId}/rows`);
}

// ─── Sync ─────────────────────────────────────────────────────────────────

/**
 * syncTimesheet sends the full timesheet payload to the backend.
 * The backend dispatches the sync asynchronously and returns 202 Accepted.
 *
 * @param {TimesheetInput} input
 * @returns {Promise<{status: string, message: string}>}
 */
export async function syncTimesheet(input) {
  return request("/api/sync", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

/**
 * pollSyncStatus fetches a one-shot snapshot of all sync statuses.
 * Used as a fallback when SSE is unavailable.
 * @returns {Promise<SyncReport>}
 */
export async function pollSyncStatus() {
  return request("/api/sync/status/poll");
}

/**
 * openSyncStatusStream opens a Server-Sent Events connection for real-time
 * sync-status updates and calls the provided callbacks.
 *
 * @param {function(SyncStatus): void} onStatus  - Called on each status update
 * @param {function(Event): void}      onError   - Called on connection error
 * @returns {EventSource} The EventSource instance – call .close() to disconnect
 */
export function openSyncStatusStream(onStatus, onError) {
  const url = `${BASE_URL}/api/sync/status`;
  const sessionId = getSessionID();

  // EventSource doesn't support custom headers directly in the browser,
  // so we append the session ID as a query parameter.  The server must
  // also accept it there for SSE connections.
  const es = new EventSource(`${url}?session=${encodeURIComponent(sessionId)}`);

  es.addEventListener("status", (e) => {
    try {
      const status = JSON.parse(e.data);
      onStatus(status);
    } catch (_) {
      /* ignore parse errors */
    }
  });

  es.onerror = onError;
  return es;
}

// ─── Health check ─────────────────────────────────────────────────────────

/**
 * healthCheck pings the backend health endpoint.
 * @returns {Promise<{status: string}>}
 */
export async function healthCheck() {
  return request("/health");
}
