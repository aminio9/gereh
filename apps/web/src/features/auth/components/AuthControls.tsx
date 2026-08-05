import { useAuth } from "../context/AuthContext";

export function AuthControls() {
  const auth = useAuth();

  if (auth.isLoading) {
    return <span>Loading session…</span>;
  }

  if (!auth.isAuthenticated) {
    return (
      <button
        type="button"
        onClick={() => {
          auth.login(window.location.pathname);
        }}
      >
        Sign in
      </button>
    );
  }

  const displayName = auth.session?.user.displayName.trim();

  const fallbackLabel = auth.session?.user.email ?? "Signed in";

  const userLabel = displayName !== undefined && displayName !== "" ? displayName : fallbackLabel;

  return (
    <div>
      <span>{userLabel}</span>

      <button
        type="button"
        onClick={() => {
          void auth.logout();
        }}
      >
        Sign out
      </button>
    </div>
  );
}
