# TimeSync Frontend

React SPA for the TimeSync application.

## Technology Stack

| Layer | Choice |
|-------|--------|
| Framework | React 18 (Hooks) |
| State | `useReducer` + Context (no Redux needed) |
| Styling | Tailwind CSS v3 |
| Build tool | Vite 5 |
| HTTP | Fetch API (no Axios) |
| Real-time | EventSource (SSE) |
| Session | `sessionStorage` UUID |

## Running Locally

```bash
# From the /frontend directory

npm install
npm run dev        # starts on http://localhost:5173
```

The Vite dev server automatically proxies `/api/*` to `http://localhost:8080`
(the Go backend). No CORS configuration needed during development.

## Building for Production

```bash
npm run build      # outputs to /dist
npm run preview    # preview the production build locally
```

## Code Organisation

```
src/
  main.jsx                     ← React entry point
  App.jsx                      ← Root component / page switcher
  index.css                    ← Tailwind imports + global styles

  context/
    AppContext.jsx              ← Global state: useReducer + Context

  services/
    api.js                     ← All backend API calls in one place
    tokenStore.js              ← Session UUID in sessionStorage

  pages/
    SetupPage.jsx              ← Step 1: select systems + authenticate
    TimesheetPage.jsx          ← Step 2: enter time + sync

  components/
    SystemSelector/
      SystemSelector.jsx       ← Card grid for choosing systems
    AuthModal/
      AuthModal.jsx            ← Dynamic credential form (driven by authFields)
    TimeTable/
      TimeTable.jsx            ← Mon–Sun entry grid with row management
    RowMapper/
      RowMapper.jsx            ← Collapsible panel to map rows to backend rows
    SyncStatus/
      SyncStatus.jsx           ← Real-time amber/green status chips
```

## State Management

All application state lives in `AppContext.jsx`. Components dispatch typed
actions via `useApp()`:

```jsx
import { useApp, ACTIONS } from '../context/AppContext';

function MyComponent() {
  const { state, dispatch } = useApp();

  // Read state
  const { rows, week, syncStatuses } = state;

  // Update state
  dispatch({ type: ACTIONS.ADD_ROW });
  dispatch({ type: ACTIONS.UPDATE_ROW_HOURS, payload: { id, day: 'monday', value: 8 } });
}
```

## Adding a New System (Frontend)

No code changes are required. Systems are loaded dynamically from
`GET /api/systems`. The auth form fields are built from the `authFields`
array in each system's response.

**Optional:** Add a custom icon in `SystemSelector.jsx`:

```js
const SYSTEM_ICONS = {
  mysystem: <svg>…</svg>,
};
```

## Security Notes

- Session ID is generated with `crypto.randomUUID()` — cryptographically strong
- Session ID is stored in `sessionStorage` — cleared when the tab closes
- Session ID is sent as `X-Session-ID` header — cannot be set automatically by browsers cross-origin (CSRF protection)
- No API tokens, passwords, or refresh tokens ever touch the browser
- The `credentials: 'omit'` fetch option ensures cookies are never sent

## Linting

```bash
npm run lint
```
