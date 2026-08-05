function readCookie(name: string): string | null {
  const prefix = `${encodeURIComponent(name)}=`;

  for (const item of document.cookie.split(";")) {
    const value = item.trim();

    if (value.startsWith(prefix)) {
      return decodeURIComponent(value.slice(prefix.length));
    }
  }

  return null;
}

export async function logout(): Promise<void> {
  const csrfToken = readCookie("gereh_csrf");

  if (!csrfToken) {
    throw new Error("CSRF token is unavailable");
  }

  const response = await fetch("/v1/auth/logout", {
    method: "POST",
    credentials: "include",
    headers: {
      "X-CSRF-Token": csrfToken,
    },
  });

  if (!response.ok && response.status !== 401) {
    throw new Error(`Failed to log out: HTTP ${response.status}`);
  }
}
