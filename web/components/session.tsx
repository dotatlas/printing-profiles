"use client";

import { useRouter } from "next/navigation";
import { createContext, type ReactNode, useCallback, useContext, useEffect, useState } from "react";
import { getMe, welcomePath } from "../lib/account";
import type { Me } from "../lib/api-types";

// SessionState is what the site knows about the visitor: still asking, signed out, or the signed-in user.
export type SessionState = { status: "loading" } | { status: "signed-out" } | { status: "signed-in"; me: Me };

// SessionValue is the state plus a way to ask the API again after a sign-in or a sign-out.
type SessionValue = { state: SessionState; refresh: () => Promise<void> };

const SessionContext = createContext<SessionValue>({ state: { status: "loading" }, refresh: async () => {} });

// SessionProvider asks the API who is signed in, once per page load, and shares the answer. The layout puts it
// around every page. The pages are the same HTML for everyone; only this browser-side call knows the visitor.
export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>({ status: "loading" });

  const refresh = useCallback(async () => {
    try {
      const me = await getMe();
      setState(me ? { status: "signed-in", me } : { status: "signed-out" });
    } catch {
      // The API could not be reached. Show the visitor as signed out; the pages that need a user say why.
      setState({ status: "signed-out" });
    }
  }, []);

  useEffect(() => {
    let live = true;
    getMe().then(
      (me) => live && setState(me ? { status: "signed-in", me } : { status: "signed-out" }),
      () => live && setState({ status: "signed-out" }),
    );
    return () => {
      live = false;
    };
  }, []);

  return <SessionContext.Provider value={{ state, refresh }}>{children}</SessionContext.Provider>;
}

// useSession gives the session state and refresh. The account menu and the account pages use it.
export function useSession(): SessionValue {
  return useContext(SessionContext);
}

// useMember gives the signed-in user who has a handle, or null while the site still asks. It sends a signed-out
// visitor to sign-in and a new user to onboarding, both with path as the way back. The account and preset pages
// that need a member call it.
export function useMember(path: string): Me | null {
  const router = useRouter();
  const { state } = useSession();
  useEffect(() => {
    if (state.status === "signed-out") router.replace(`/signin?return=${encodeURIComponent(path)}`);
    if (state.status === "signed-in" && state.me.needs_onboarding) router.replace(welcomePath(path));
  }, [state, router, path]);
  return state.status === "signed-in" && !state.me.needs_onboarding ? state.me : null;
}
