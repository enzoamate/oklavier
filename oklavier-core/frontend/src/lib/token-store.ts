// Auth tokens are now httpOnly cookies set by the Go backend (login/refresh
// flows). They are NEVER accessible from JavaScript — that is the entire
// point: an XSS can no longer exfiltrate the long-lived refresh token.
//
// These functions are kept as no-ops to avoid breaking imports during the
// migration. They will be removed once all consumers stop calling them.

export function getAccessToken(): string | null {
  return null;
}

export function getRefreshToken(): string | null {
  return null;
}

export function setTokens(_access: string, _refresh: string): void {
  // No-op: cookies are set by the backend Set-Cookie response.
}

export function clearTokens(): void {
  // No-op: logout endpoint clears the cookies.
}
