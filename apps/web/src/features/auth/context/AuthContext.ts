import { createContext, useContext } from "react";

import type { AuthSession } from "../api/getSession";

export interface AuthContextValue {
  session: AuthSession | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (returnTo?: string) => void;
  logout: () => Promise<void>;
}

export const authSessionQueryKey = ["auth", "session"] as const;

export const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);

  if (value === null) {
    throw new Error("useAuth must be used inside AuthProvider");
  }

  return value;
}
