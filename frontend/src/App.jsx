/**
 * App.jsx
 *
 * Root application component.
 * Renders SetupPage or TimesheetPage based on the current page state.
 */

import React from "react";
import { AppProvider, useApp } from "./context/AppContext";
import SetupPage from "./pages/SetupPage";
import TimesheetPage from "./pages/TimesheetPage";

function AppContent() {
  const { state } = useApp();
  return state.page === "timesheet" ? <TimesheetPage /> : <SetupPage />;
}

export default function App() {
  return (
    <AppProvider>
      <AppContent />
    </AppProvider>
  );
}
