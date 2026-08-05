import { useCallback, useMemo, type PropsWithChildren } from "react";

import { useQuery, useQueryClient } from "@tanstack/react-query";

import { getSession } from "../api/getSession";
import { logout as logoutRequest } from "../api/logout";
import { AuthContext, authSessionQueryKey, type AuthContextValue } from "./AuthContext";

export function AuthProvider({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();

  const sessionQuery = useQuery({
    queryKey: authSessionQueryKey,
    queryFn: ({ signal }) => getSession(signal),
    retry: false,
    staleTime: 30_000,
    refetchOnWindowFocus: true,
  });

  const login = useCallback((returnTo = "/") => {
    const target = new URL("/v1/auth/login", window.location.origin);

    target.searchParams.set("return_to", returnTo);

    window.location.assign(target);
  }, []);

  const logout = useCallback(async () => {
    await logoutRequest();

    queryClient.setQueryData(authSessionQueryKey, null);
  }, [queryClient]);

  const value = useMemo<AuthContextValue>(
    () => ({
      session: sessionQuery.data ?? null,
      isLoading: sessionQuery.isLoading,
      isAuthenticated: sessionQuery.data != null,
      login,
      logout,
    }),
    [login, logout, sessionQuery.data, sessionQuery.isLoading],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
