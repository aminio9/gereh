import { createBrowserRouter, Navigate } from "react-router";

import { ActivityPage } from "../features/dashboard/pages/ActivityPage";
import { AgentsPage } from "../features/dashboard/pages/AgentsPage";
import { DashboardPage } from "../features/dashboard/pages/DashboardPage";
import { SearchPage } from "../features/dashboard/pages/SearchPage";
import { AppShell } from "../features/shell/AppShell";
import { NotFoundPage } from "../pages/NotFoundPage";
import { RouteErrorPage } from "../pages/RouteErrorPage";
import { RootLayout } from "./RootLayout";
import { WorkspaceEntryPage } from "./WorkspaceEntryPage";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <RootLayout />,
    errorElement: <RouteErrorPage />,
    children: [
      {
        index: true,
        element: <WorkspaceEntryPage />,
      },
      {
        path: "t/:tenantId",
        element: <AppShell />,
        children: [
          {
            index: true,
            element: <Navigate replace to="dashboard" />,
          },
          {
            path: "dashboard",
            element: <DashboardPage />,
          },
          {
            path: "agents",
            element: <AgentsPage />,
          },
          {
            path: "activity",
            element: <ActivityPage />,
          },
          {
            path: "search",
            element: <SearchPage />,
          },
        ],
      },
      {
        path: "*",
        element: <NotFoundPage />,
      },
    ],
  },
]);
