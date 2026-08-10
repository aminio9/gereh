import { useQuery } from "@tanstack/react-query";
import { Navigate } from "react-router";

import { LoginPage } from "../features/auth/pages/LoginPage";
import { useAuth } from "../features/auth/context/AuthContext";
import { tenantsQueryOptions } from "../features/tenants/api/listTenants";

export function WorkspaceEntryPage() {
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  const tenantsQuery = useQuery(tenantsQueryOptions(isAuthenticated));

  if (authLoading) {
    return <div className="loading-screen">در حال لود سشن…</div>;
  }

  if (!isAuthenticated) {
    return <LoginPage />;
  }

  if (tenantsQuery.isLoading) {
    return <div className="loading-screen">در حال لود تننت…</div>;
  }

  if (tenantsQuery.isError) {
    return (
      <main className="empty-workspace">
        <h1>تننت لود نشد</h1>
        <p>ارتباط با تننت سرویس را بررسی کنید و رفرش کنید.</p>
      </main>
    );
  }

  const firstTenant = tenantsQuery.data?.contexts[0]?.tenant;

  if (!firstTenant) {
    return (
      <main className="empty-workspace">
        <div className="empty-state">
          <h1>هنوز تننتی ندارید</h1>
          <p>
            ابتدا فلوی آنبوردینگ تننت را کامل کنید؛ سپس داشبورد از پروجکشن‌های واقعی نمایش داده
            می‌شود.
          </p>
        </div>
      </main>
    );
  }

  return <Navigate replace to={`/t/${encodeURIComponent(firstTenant.tenantId)}/dashboard`} />;
}
