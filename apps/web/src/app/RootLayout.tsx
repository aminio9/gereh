import { Outlet } from "react-router";

export function RootLayout() {
  return (
    <div className="application">
      <Outlet />
    </div>
  );
}
