import { Menu, PanelRight, Search } from "lucide-react";
import { Navigate, Outlet, useLocation, useNavigate } from "react-router";
import { useMemo, useState, type FormEvent } from "react";

import { useAuth } from "../auth/context/AuthContext";
import { useTenantWorkspace } from "../tenants/hooks/useTenantWorkspace";
import { Sidebar } from "./Sidebar";

const sidebarStorageKey = "gereh.sidebar.collapsed";

function pageTitle(pathname: string): string {
  if (pathname.endsWith("/agents")) {
    return "ایجنت‌ها";
  }

  if (pathname.endsWith("/activity")) {
    return "اکتیویتی";
  }

  if (pathname.endsWith("/search")) {
    return "سرچ";
  }

  return "داشبورد";
}

export function AppShell() {
  const location = useLocation();
  const navigate = useNavigate();

  const { session, isAuthenticated, isLoading: authLoading, logout } = useAuth();

  const {
    tenantId,
    tenantContexts,
    selectedTenant,
    isLoading: tenantLoading,
    error: tenantError,
  } = useTenantWorkspace();

  const [collapsed, setCollapsed] = useState(() => {
    return localStorage.getItem(sidebarStorageKey) === "true";
  });

  const [mobileOpen, setMobileOpen] = useState(false);
  const [searchValue, setSearchValue] = useState("");

  const [previousPathname, setPreviousPathname] = useState(location.pathname);

  if (previousPathname !== location.pathname) {
    setPreviousPathname(location.pathname);
    setMobileOpen(false);
  }

  const title = useMemo(() => pageTitle(location.pathname), [location.pathname]);

  if (authLoading || tenantLoading) {
    return <div className="loading-screen">در حال لود ورک‌اسپیس…</div>;
  }

  if (!isAuthenticated || !session) {
    return <Navigate replace to="/" />;
  }

  if (tenantError) {
    return (
      <main className="empty-workspace">
        <h1>تننت لود نشد</h1>
        <p>لطفاً ارتباط BFF و تننت سرویس را بررسی کنید.</p>
      </main>
    );
  }

  if (!selectedTenant) {
    return <Navigate replace to="/" />;
  }

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      const next = !current;

      localStorage.setItem(sidebarStorageKey, String(next));

      return next;
    });
  };

  const submitSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const query = searchValue.trim();

    if (query.length < 2) {
      return;
    }

    void navigate(`/t/${encodeURIComponent(tenantId)}/search?q=${encodeURIComponent(query)}`);
  };

  return (
    <div className="app-shell" data-sidebar-collapsed={collapsed}>
      <a className="skip-link" href="#main-content">
        پرش به محتوای اصلی
      </a>

      <div className="content-shell">
        <header className="app-topbar">
          <div className="topbar-title">
            <button
              className="icon-button mobile-sidebar-button"
              type="button"
              aria-label="باز کردن سایدبار"
              onClick={() => setMobileOpen(true)}
            >
              <Menu size={18} />
            </button>

            <div>
              <span className="topbar-eyebrow">
                {selectedTenant.displayName || selectedTenant.slug}
              </span>

              <h1>{title}</h1>
            </div>
          </div>

          <form className="topbar-search" role="search" onSubmit={submitSearch}>
            <Search size={16} aria-hidden="true" />

            <input
              value={searchValue}
              onChange={(event) => setSearchValue(event.target.value)}
              placeholder="سرچ در کمپانی، ایجنت، پروجکت و تسک…"
              aria-label="سرچ"
            />

            <kbd>/</kbd>
          </form>

          <button
            className="icon-button desktop-sidebar-button"
            type="button"
            aria-label={collapsed ? "باز کردن سایدبار" : "بستن سایدبار"}
            onClick={toggleCollapsed}
          >
            <PanelRight size={17} />
          </button>
        </header>

        <main id="main-content" className="page-content" tabIndex={-1}>
          <Outlet />
        </main>
      </div>

      <Sidebar
        tenantId={tenantId}
        tenants={tenantContexts}
        user={session.user}
        collapsed={collapsed}
        mobileOpen={mobileOpen}
        onToggleCollapsed={toggleCollapsed}
        onCloseMobile={() => setMobileOpen(false)}
        onLogout={logout}
      />

      {mobileOpen && (
        <button
          type="button"
          className="sidebar-backdrop"
          aria-label="بستن سایدبار"
          onClick={() => setMobileOpen(false)}
        />
      )}
    </div>
  );
}
