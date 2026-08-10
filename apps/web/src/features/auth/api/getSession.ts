import { environment } from "../../../config/env";

export interface AuthenticatedUser {
  userId: string;
  issuer: string;
  subject: string;
  email: string;
  emailVerified: boolean;
  displayName: string;
  pictureUrl: string;
}

export interface AuthSession {
  user: AuthenticatedUser;
  createdAt: string;
  expiresAt: string;
}

export async function getSession(signal?: AbortSignal): Promise<AuthSession | null> {
  const response = await fetch(`${environment.apiBaseUrl}/v1/auth/session`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
    signal: signal ?? null,
  });

  if (response.status === 401) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Failed to retrieve session: HTTP ${response.status}`);
  }

  return (await response.json()) as AuthSession;
}
