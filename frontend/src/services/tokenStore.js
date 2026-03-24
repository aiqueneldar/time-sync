/**
 * tokenStore.js
 *
 * Manages the browser-side session identity for TimeSync.
 *
 * Security design
 * ───────────────
 * • A random UUID v4 is generated once per browser tab and stored in
 *   sessionStorage.  It is cleared automatically when the tab is closed.
 * • This ID is sent as the X-Session-ID header on every API request.
 *   The *actual* API tokens never leave the backend server – only the
 *   session ID is held in the browser.
 * • Using sessionStorage (not localStorage) means tokens cannot be accessed
 *   by other tabs, and the session expires naturally when the tab closes.
 *
 * OWASP: A02 Cryptographic Failures – sensitive data handled in backend RAM only.
 */

const SESSION_KEY = "timesync_session_id";

/**
 * generateUUIDv4 creates a version-4 UUID using the Web Crypto API.
 * This is cryptographically strong and works in all modern browsers.
 * @returns {string}
 */
function generateUUIDv4() {
  // Use crypto.randomUUID() if available (Chrome 92+, Firefox 95+, Safari 15.4+)
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  // Fallback: manual construction via getRandomValues.
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // variant bits
  const hex = [...bytes].map((b) => b.toString(16).padStart(2, "0")).join("");
  return [
    hex.slice(0, 8),
    hex.slice(8, 12),
    hex.slice(12, 16),
    hex.slice(16, 20),
    hex.slice(20),
  ].join("-");
}

/**
 * getSessionID returns the current session ID, creating one if it does not
 * yet exist in sessionStorage.
 * @returns {string} UUID v4
 */
export function getSessionID() {
  let id = sessionStorage.getItem(SESSION_KEY);
  if (!id) {
    id = generateUUIDv4();
    sessionStorage.setItem(SESSION_KEY, id);
  }
  return id;
}

/**
 * clearSession removes the session ID from sessionStorage and reloads the
 * page, effectively logging the user out.
 */
export function clearSession() {
  sessionStorage.removeItem(SESSION_KEY);
}

/**
 * hasSession returns true if a session ID has already been created.
 * Used to determine whether to show the setup flow.
 * @returns {boolean}
 */
export function hasSession() {
  return !!sessionStorage.getItem(SESSION_KEY);
}
