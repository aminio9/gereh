import {
  Activity,
  Bot,
  Building2,
  FolderKanban,
  LayoutDashboard,
  ListTodo,
  LogOut,
  Network,
  PanelRightClose,
  PanelRightOpen,
  Search,
  ShieldCheck,
} from "lucide-react";
import { NavLink, useNavigate } from "react-router";

import type { AuthenticatedUser } from "../auth/api/getSession";
import type { TenantContext } from "../tenants/api/listTenants";

interface SidebarProps {
  tenantId: string;
  tenants: TenantContext[];
  user: AuthenticatedUser;
  collapsed: boolean;
  mobileOpen: boolean;
  onToggleCollapsed: () => void;
  onCloseMobile: () => void;
  onLogout: () => Promise<void>;
}

export function Sidebar({
  tenantId,
  tenants,
  user,
  collapsed,
  mobileOpen,
  onToggleCollapsed,
  onCloseMobile,
  onLogout,
}: SidebarProps) {
  const navigate = useNavigate();

  const basePath = `/t/${encodeURIComponent(tenantId)}`;

  const tenant = tenants.find((context) => context.tenant.tenantId === tenantId)?.tenant;

  const navigateTenant = (nextTenantId: string) => {
    void navigate(`/t/${encodeURIComponent(nextTenantId)}/dashboard`);

    onCloseMobile();
  };

  return (
    <aside
      className="app-sidebar"
      data-collapsed={collapsed}
      data-mobile-open={mobileOpen}
      aria-label="ناوبری اصلی"
    >
      <div className="sidebar-top">
        <div className="sidebar-brand">
          <div className="brand-mark" aria-hidden="true">
            <Network size={18} />
          </div>

          {!collapsed && (
            <div className="sidebar-brand__text">
              <strong>گِره</strong>
              <span>کنترل پلین</span>
            </div>
          )}
        </div>

        <button
          type="button"
          className="sidebar-collapse"
          aria-label={collapsed ? "باز کردن سایدبار" : "بستن سایدبار"}
          aria-expanded={!collapsed}
          onClick={onToggleCollapsed}
        >
          {collapsed ? <PanelRightOpen size={17} /> : <PanelRightClose size={17} />}
        </button>
      </div>

      {!collapsed && (
        <label className="tenant-switcher">
          <span>تننت</span>

          <select value={tenantId} onChange={(event) => navigateTenant(event.target.value)}>
            {tenants.map((context) => (
              <option key={context.tenant.tenantId} value={context.tenant.tenantId}>
                {context.tenant.displayName || context.tenant.slug}
              </option>
            ))}
          </select>
        </label>
      )}

      {collapsed && (
        <div className="tenant-monogram" title={tenant?.displayName ?? tenant?.slug}>
          {(tenant?.displayName ?? tenant?.slug ?? "ت").slice(0, 1).toUpperCase()}
        </div>
      )}

      <nav className="sidebar-nav">
        <div className="nav-section">
          {!collapsed && <p className="nav-section__label">ورک‌اسپیس</p>}

          <NavLink
            to={`${basePath}/search`}
            className={({ isActive }) => `nav-item${isActive ? " nav-item--active" : ""}`}
            onClick={onCloseMobile}
          >
            <Search size={17} />
            {!collapsed && <span>سرچ</span>}
          </NavLink>

          <NavLink
            to={`${basePath}/dashboard`}
            className={({ isActive }) => `nav-item${isActive ? " nav-item--active" : ""}`}
            onClick={onCloseMobile}
          >
            <LayoutDashboard size={17} />
            {!collapsed && <span>داشبورد</span>}
          </NavLink>

          <NavLink
            to={`${basePath}/agents`}
            className={({ isActive }) => `nav-item${isActive ? " nav-item--active" : ""}`}
            onClick={onCloseMobile}
          >
            <Bot size={17} />
            {!collapsed && <span>ایجنت‌ها</span>}
          </NavLink>

          <NavLink
            to={`${basePath}/activity`}
            className={({ isActive }) => `nav-item${isActive ? " nav-item--active" : ""}`}
            onClick={onCloseMobile}
          >
            <Activity size={17} />
            {!collapsed && <span>اکتیویتی</span>}
          </NavLink>
        </div>

        <div className="nav-section">
          {!collapsed && <p className="nav-section__label">کور</p>}

          <button
            type="button"
            className="nav-item nav-item--disabled"
            disabled
            title="UI این فاز هنوز ساخته نشده"
          >
            <Building2 size={17} />
            {!collapsed && <span>کمپانی‌ها</span>}
          </button>

          <button
            type="button"
            className="nav-item nav-item--disabled"
            disabled
            title="UI این فاز هنوز ساخته نشده"
          >
            <FolderKanban size={17} />
            {!collapsed && <span>پروجکت‌ها</span>}
          </button>

          <button
            type="button"
            className="nav-item nav-item--disabled"
            disabled
            title="UI این فاز هنوز ساخته نشده"
          >
            <ListTodo size={17} />
            {!collapsed && <span>تسک‌ها</span>}
          </button>

          <button
            type="button"
            className="nav-item nav-item--disabled"
            disabled
            title="UI این فاز هنوز ساخته نشده"
          >
            <ShieldCheck size={17} />
            {!collapsed && <span>پالیسی</span>}
          </button>
        </div>
      </nav>

      <div className="sidebar-user">
        {user.pictureUrl ? (
          <img
            className="sidebar-user__avatar"
            src={user.pictureUrl}
            alt=""
            referrerPolicy="no-referrer"
          />
        ) : (
          <div className="sidebar-user__avatar sidebar-user__avatar--fallback">
            {(user.displayName || user.email).slice(0, 1).toUpperCase()}
          </div>
        )}

        {!collapsed && (
          <div className="sidebar-user__identity">
            <strong>{user.displayName || "یوزر"}</strong>
            <span dir="ltr">{user.email}</span>
          </div>
        )}

        <button
          className="icon-button"
          type="button"
          title="لاگ‌اوت"
          aria-label="لاگ‌اوت"
          onClick={() => void onLogout()}
        >
          <LogOut size={16} />
        </button>
      </div>
    </aside>
  );
}
