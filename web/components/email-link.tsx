"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, useSyncExternalStore } from "react";
import { ApiError, finishEmailSignIn, safeReturn, waitText } from "../lib/account";
import { Notice } from "./account-ui";
import { useSession } from "./session";

type LinkState = { step: "ready" } | { step: "signing" } | { step: "dead" } | { step: "error"; message: string };

let linkToken: string | null | undefined;

// readLinkToken takes the token from the URL's fragment one time, then removes it from the address bar and the
// history, so the back button or a shared screen cannot show it again. EmailLink reads it through React.
function readLinkToken(): string | null {
  if (linkToken === undefined) {
    linkToken = new URLSearchParams(window.location.hash.slice(1)).get("token");
    window.history.replaceState(null, "", window.location.pathname);
  }
  return linkToken;
}

// noSubscribe is the subscribe function for a value that never changes after the first read.
function noSubscribe() {
  return () => {};
}

// EmailLink finishes an email sign-in. It takes the token from the URL's fragment (which no server ever sees),
// removes it from the address bar, and signs in only when the visitor presses the button, because mail scanners
// open links on their own. The /auth/email route renders it.
export function EmailLink() {
  const router = useRouter();
  const { refresh } = useSession();
  // On the server there is no URL fragment yet, so the token is undefined until the browser reads it.
  const token = useSyncExternalStore(noSubscribe, readLinkToken, () => undefined);
  const [linkState, setState] = useState<LinkState>({ step: "ready" });
  const state: LinkState | { step: "reading" } = token === undefined ? { step: "reading" } : token === null ? { step: "dead" } : linkState;

  async function onSignIn(token: string) {
    setState({ step: "signing" });
    try {
      const { return: next } = await finishEmailSignIn(token);
      await refresh();
      router.replace(safeReturn(next));
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        setState({ step: "dead" });
      } else if (err instanceof ApiError && err.status === 429) {
        setState({ step: "error", message: `Too many attempts. Try again ${waitText(err.retryAfter)}.` });
      } else {
        const message = err instanceof ApiError && err.status === 0 ? err.detail : "Something went wrong. Try again.";
        setState({ step: "error", message });
      }
    }
  }

  return (
    <div className="mx-auto max-w-sm">
      <h1 className="text-2xl font-semibold tracking-tight">Finish signing in</h1>
      {state.step === "dead" ? (
        <>
          <div className="mt-6">
            <Notice tone="error">
              This sign-in link has expired or was already used. A link works once, for 15 minutes.
            </Notice>
          </div>
          <Link href="/signin" className="btn btn-primary mt-6 w-full">
            Get a new link
          </Link>
        </>
      ) : (
        <>
          <p className="mt-2 text-muted">Press the button to sign in to Printing Profiles. The link works once.</p>
          {state.step === "error" && (
            <div className="mt-6">
              <Notice tone="error">{state.message}</Notice>
            </div>
          )}
          <button
            type="button"
            className="btn btn-primary mt-6 w-full"
            disabled={state.step === "reading" || state.step === "signing"}
            onClick={() => token && onSignIn(token)}
          >
            {state.step === "signing" ? "Signing in…" : "Sign in"}
          </button>
        </>
      )}
    </div>
  );
}
