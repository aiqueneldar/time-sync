/**
 * AppContext.jsx
 *
 * Single source of truth for all TimeSync application state.
 *
 * Architecture:
 * ─────────────
 * useReducer drives all state transitions so changes are predictable,
 * testable, and traceable.  The dispatch function is exposed via context
 * so any component can trigger state changes without prop drilling.
 *
 * State shape:
 * ─────────────────────────────────────────────────────────────────────
 * {
 *   // Setup phase
 *   systems:          SystemInfo[]        – all available systems from backend
 *   selectedSystems:  string[]            – IDs of systems user chose
 *   authStatuses:     Record<id, AuthStatus>
 *
 *   // Time entry phase
 *   week:             { year, week }      – currently selected ISO week
 *   rows:             TimeEntryRow[]      – user's time-entry table
 *   availableRows:    Record<id, SystemRow[]> – rows fetched per system
 *
 *   // Sync phase
 *   syncStatuses:     Record<id, SyncStatus>
 *   isSyncing:        boolean
 *
 *   // UI state
 *   page:             'setup' | 'timesheet'
 *   loadingStates:    Record<string, boolean>
 *   errors:           Record<string, string>
 * }
 */

import React, {
  createContext, useContext, useReducer, useCallback,
} from 'react';
import { v4 as uuidv4 } from 'uuid';

// ─── Initial state ────────────────────────────────────────────────────────

function getInitialWeek() {
  const now = new Date();
  const jan4 = new Date(now.getFullYear(), 0, 4);
  const startOfWeek1 = new Date(jan4);
  startOfWeek1.setDate(jan4.getDate() - ((jan4.getDay() + 6) % 7));
  const diff = now - startOfWeek1;
  const week = Math.floor(diff / (7 * 24 * 60 * 60 * 1000)) + 1;
  return { year: now.getFullYear(), week };
}

const initialState = {
  // Setup
  systems: [],
  selectedSystems: [],
  authStatuses: {},

  // Timesheet
  week: getInitialWeek(),
  rows: [
    // Start with one empty row.
    {
      id: uuidv4(),
      label: '',
      hours: { monday: 0, tuesday: 0, wednesday: 0, thursday: 0, friday: 0, saturday: 0, sunday: 0 },
      mappings: [],
    },
  ],
  availableRows: {}, // { [systemId]: SystemRow[] }

  // Sync
  syncStatuses: {},  // { [systemId]: SyncStatus }
  isSyncing: false,

  // UI
  page: 'setup',
  loadingStates: {},
  errors: {},
};

// ─── Action types ─────────────────────────────────────────────────────────

export const ACTIONS = {
  // Systems
  SET_SYSTEMS:          'SET_SYSTEMS',
  SET_SELECTED_SYSTEMS: 'SET_SELECTED_SYSTEMS',

  // Auth
  SET_AUTH_STATUS:      'SET_AUTH_STATUS',

  // Navigation
  SET_PAGE:             'SET_PAGE',

  // Week navigation
  SET_WEEK:             'SET_WEEK',

  // Time-entry rows
  ADD_ROW:              'ADD_ROW',
  REMOVE_ROW:           'REMOVE_ROW',
  UPDATE_ROW_LABEL:     'UPDATE_ROW_LABEL',
  UPDATE_ROW_HOURS:     'UPDATE_ROW_HOURS',
  UPDATE_ROW_MAPPINGS:  'UPDATE_ROW_MAPPINGS',

  // Available rows from backend systems
  SET_AVAILABLE_ROWS:   'SET_AVAILABLE_ROWS',

  // Sync
  SET_SYNC_STATUS:      'SET_SYNC_STATUS',
  SET_IS_SYNCING:       'SET_IS_SYNCING',

  // UI helpers
  SET_LOADING:          'SET_LOADING',
  SET_ERROR:            'SET_ERROR',
  CLEAR_ERROR:          'CLEAR_ERROR',
};

// ─── Reducer ─────────────────────────────────────────────────────────────

function reducer(state, action) {
  switch (action.type) {

    case ACTIONS.SET_SYSTEMS:
      return { ...state, systems: action.payload };

    case ACTIONS.SET_SELECTED_SYSTEMS:
      return { ...state, selectedSystems: action.payload };

    case ACTIONS.SET_AUTH_STATUS:
      return {
        ...state,
        authStatuses: {
          ...state.authStatuses,
          [action.payload.systemId]: action.payload,
        },
      };

    case ACTIONS.SET_PAGE:
      return { ...state, page: action.payload };

    case ACTIONS.SET_WEEK:
      return { ...state, week: action.payload };

    case ACTIONS.ADD_ROW:
      return {
        ...state,
        rows: [
          ...state.rows,
          {
            id: uuidv4(),
            label: '',
            hours: { monday: 0, tuesday: 0, wednesday: 0, thursday: 0, friday: 0, saturday: 0, sunday: 0 },
            mappings: [],
          },
        ],
      };

    case ACTIONS.REMOVE_ROW:
      return {
        ...state,
        rows: state.rows.filter(r => r.id !== action.payload),
      };

    case ACTIONS.UPDATE_ROW_LABEL:
      return {
        ...state,
        rows: state.rows.map(r =>
          r.id === action.payload.id ? { ...r, label: action.payload.label } : r
        ),
      };

    case ACTIONS.UPDATE_ROW_HOURS:
      return {
        ...state,
        rows: state.rows.map(r =>
          r.id === action.payload.id
            ? { ...r, hours: { ...r.hours, [action.payload.day]: action.payload.value } }
            : r
        ),
      };

    case ACTIONS.UPDATE_ROW_MAPPINGS:
      return {
        ...state,
        rows: state.rows.map(r =>
          r.id === action.payload.id ? { ...r, mappings: action.payload.mappings } : r
        ),
      };

    case ACTIONS.SET_AVAILABLE_ROWS:
      return {
        ...state,
        availableRows: {
          ...state.availableRows,
          [action.payload.systemId]: action.payload.rows,
        },
      };

    case ACTIONS.SET_SYNC_STATUS:
      return {
        ...state,
        syncStatuses: {
          ...state.syncStatuses,
          [action.payload.systemId]: action.payload,
        },
      };

    case ACTIONS.SET_IS_SYNCING:
      return { ...state, isSyncing: action.payload };

    case ACTIONS.SET_LOADING:
      return {
        ...state,
        loadingStates: {
          ...state.loadingStates,
          [action.payload.key]: action.payload.value,
        },
      };

    case ACTIONS.SET_ERROR:
      return {
        ...state,
        errors: {
          ...state.errors,
          [action.payload.key]: action.payload.message,
        },
      };

    case ACTIONS.CLEAR_ERROR: {
      const { [action.payload]: _removed, ...rest } = state.errors;
      return { ...state, errors: rest };
    }

    default:
      return state;
  }
}

// ─── Context ──────────────────────────────────────────────────────────────

const AppContext = createContext(null);

/**
 * AppProvider wraps the application and provides the global state.
 */
export function AppProvider({ children }) {
  const [state, dispatch] = useReducer(reducer, initialState);

  // Convenience helpers so components don't need to import ACTIONS.
  const setLoading = useCallback((key, value) =>
    dispatch({ type: ACTIONS.SET_LOADING, payload: { key, value } }), []);

  const setError = useCallback((key, message) =>
    dispatch({ type: ACTIONS.SET_ERROR, payload: { key, message } }), []);

  const clearError = useCallback((key) =>
    dispatch({ type: ACTIONS.CLEAR_ERROR, payload: key }), []);

  return (
    <AppContext.Provider value={{ state, dispatch, setLoading, setError, clearError }}>
      {children}
    </AppContext.Provider>
  );
}

/**
 * useApp returns the app state and dispatch from the nearest AppProvider.
 * Throws if used outside of AppProvider.
 */
export function useApp() {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error('useApp must be used within an AppProvider');
  return ctx;
}

export default AppContext;