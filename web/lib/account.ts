import type { Identity, Me, Session } from "./api-types";

// apiBase is the API's URL for calls from the browser. next.config.ts puts it in the build from API_URL.
export const apiBase = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// ApiError is a failed API call: the status, the problem detail, and the seconds in Retry-After when the API sent it.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly detail: string,
    readonly retryAfter: number | null,
  ) {
    super(detail || `status ${status}`);
  }
}

// call sends one request to the API with the session cookie, and gives the JSON answer or throws ApiError.
// Every function below and lib/my-presets.ts use it. The browser adds the site's Origin header, which the API checks
// on writes.
export async function call<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(apiBase + path, {
      method,
      credentials: "include",
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "The site could not reach the server. Check your connection and try again.", null);
  }
  if (!res.ok) {
    const problem: { detail?: string } = await res.json().catch(() => ({}));
    const retry = Number(res.headers.get("Retry-After"));
    throw new ApiError(res.status, problem.detail ?? "", Number.isFinite(retry) && retry > 0 ? retry : null);
  }
  return (res.status === 204 ? undefined : await res.json()) as T;
}

// Provider is a sign-in provider with an OAuth app.
export type Provider = "github" | "discord";

// oauthStartURL gives the API route that starts a GitHub or Discord sign-in. A link to it is a full page navigation.
export function oauthStartURL(provider: Provider, returnPath: string, link = false): string {
  const q = new URLSearchParams({ return: returnPath });
  if (link) q.set("link", "1");
  return `${apiBase}/api/v1/auth/${provider}/start?${q}`;
}

// getMe gives the signed-in user, or null when nobody is signed in. The session provider calls it.
export async function getMe(): Promise<Me | null> {
  try {
    return await call<Me>("/api/v1/me");
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
}

// startEmailSignIn asks the API to email a one-time sign-in link. The sign-in page calls it.
export function startEmailSignIn(email: string, returnPath: string) {
  return call<{ status: string }>("/api/v1/auth/email/start", "POST", { email, return: returnPath });
}

// finishEmailSignIn signs in with the token from an email link and gives the path to go to. The link page calls it.
export function finishEmailSignIn(token: string) {
  return call<{ return: string }>("/api/v1/auth/email/finish", "POST", { token });
}

// finishOnboarding sets the new user's handle and accepts the terms. The welcome page calls it.
export function finishOnboarding(handle: string) {
  return call<Me>("/api/v1/me/onboarding", "POST", { handle, accept_terms: true });
}

// updateDisplayName changes the signed-in user's display name. The settings page calls it.
export function updateDisplayName(displayName: string) {
  return call<Me>("/api/v1/me", "PATCH", { display_name: displayName });
}

// listSessions gives the signed-in user's sessions, the most recent first. The settings page calls it.
export function listSessions() {
  return call<{ items: Session[] }>("/api/v1/me/sessions").then((r) => r.items);
}

// endSession signs out one session of the signed-in user. The settings page calls it.
export function endSession(id: number) {
  return call<void>(`/api/v1/me/sessions/${id}`, "DELETE");
}

// listIdentities gives the ways the signed-in user can sign in. The settings page calls it.
export function listIdentities() {
  return call<{ items: Identity[] }>("/api/v1/me/identities").then((r) => r.items);
}

// removeIdentity removes one sign-in method; the API refuses the last one. The settings page calls it.
export function removeIdentity(id: number) {
  return call<void>(`/api/v1/me/identities/${id}`, "DELETE");
}

// signOut ends this session. The account menu calls it.
export function signOut() {
  return call<void>("/api/v1/auth/signout", "POST");
}

// signOutOthers ends every session of the signed-in user except this one. The settings page calls it.
export function signOutOthers() {
  return call<{ signed_out: number }>("/api/v1/auth/signout-others", "POST");
}

// signOutEverywhere ends every session of the signed-in user. The settings page calls it.
export function signOutEverywhere() {
  return call<void>("/api/v1/auth/signout-all", "POST");
}

// safeReturn gives p when it is a plain path on this site, or "/" otherwise. It mirrors the API's own check,
// so a return path from the URL never sends the browser to another site.
export function safeReturn(p: string | null): string {
  if (!p || !p.startsWith("/") || p.startsWith("//") || p.includes("\\") || p.length > 200) return "/";
  return /[\u0000-\u001f\u007f]/.test(p) ? "/" : p;
}

// welcomePath gives the onboarding page with the final return path. Every sign-in goes through it, and it
// forwards a user who already has a handle at once.
export function welcomePath(returnPath: string): string {
  return `/welcome?${new URLSearchParams({ return: safeReturn(returnPath) })}`;
}

// waitText turns a Retry-After in seconds into words, such as "in 12 minutes". The error messages use it.
export function waitText(seconds: number | null): string {
  if (!seconds) return "later";
  const minutes = Math.ceil(seconds / 60);
  return minutes <= 1 ? "in a minute" : `in ${minutes} minutes`;
}
