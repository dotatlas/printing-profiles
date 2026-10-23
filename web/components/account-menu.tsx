"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useId, useRef, useState } from "react";
import { signOut, welcomePath } from "../lib/account";
import { useSession } from "./session";

// AccountMenu is the header's account control: "Sign in" when signed out, or the handle with a small menu
// (Settings, Sign out) when signed in. The layout puts it in the header beside the theme switch.
export function AccountMenu() {
  const { state, refresh } = useSession();
  const pathname = usePathname();
  const router = useRouter();
  // The menu is open only on the page where it was opened, so a new page closes it with no extra work.
  const [openOn, setOpenOn] = useState<string | null>(null);
  const open = openOn === pathname;
  const setOpen = (value: boolean) => setOpenOn(value ? pathname : null);
  const [busy, setBusy] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const menuId = useId();

  // Escape and a click outside close the menu. Escape also puts the focus back on the button.
  useEffect(() => {
    if (!open) return;
    const close = () => setOpenOn(null);
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        close();
        button.current?.focus();
      }
    }
    function onPointer(e: PointerEvent) {
      if (!root.current?.contains(e.target as Node)) close();
    }
    // Tab past the last item also closes the menu, so it never stays open over the page.
    function onFocus(e: FocusEvent) {
      if (!root.current?.contains(e.target as Node)) close();
    }
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("focusin", onFocus);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("focusin", onFocus);
    };
  }, [open]);

  // While the site asks the API, the slot keeps its width, so the header does not jump.
  if (state.status === "loading") return <div className="h-10 w-20" aria-hidden="true" />;

  if (state.status === "signed-out") {
    // The sign-in pages are the sign-in, so a second "Sign in" there would only reset the return path.
    if (pathname.startsWith("/signin") || pathname.startsWith("/auth")) return null;
    const here = pathname;
    return (
      <Link href={`/signin?${new URLSearchParams({ return: here })}`} className="btn btn-secondary">
        Sign in
      </Link>
    );
  }

  const { me } = state;
  const label = me.needs_onboarding ? "Finish setup" : `@${me.handle}`;

  async function onSignOut() {
    setBusy(true);
    try {
      await signOut();
    } finally {
      setBusy(false);
      setOpen(false);
      await refresh();
      router.push("/");
    }
  }

  return (
    <div ref={root} className="relative">
      <button
        ref={button}
        type="button"
        aria-expanded={open}
        aria-controls={menuId}
        onClick={() => setOpen(!open)}
        title={label}
        className="btn btn-secondary max-w-34 px-3 sm:max-w-56 sm:px-4"
      >
        <span className="sr-only">Account menu: </span>
        <span className="truncate">{label}</span>
        <svg viewBox="0 0 16 16" className="size-3.5 shrink-0 text-muted" aria-hidden="true">
          <path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      <div
        id={menuId}
        data-open={open}
        inert={!open}
        className="account-menu fixed inset-x-4 top-16 z-10 rounded-xl border border-line bg-page p-1 sm:absolute sm:inset-x-auto sm:top-full sm:right-0 sm:mt-2 sm:w-56"
      >
        {me.display_name && (
          <p className="truncate px-3 pt-2 pb-1 text-sm text-muted" title={me.display_name}>
            {me.display_name}
          </p>
        )}
        <ul className="flex flex-col">
          {me.needs_onboarding && (
            <li>
              <Link href={welcomePath(pathname)} className="menu-item">
                Choose your handle
              </Link>
            </li>
          )}
          <li>
            <Link href="/settings" className="menu-item">
              Settings
            </Link>
          </li>
          <li>
            <button type="button" onClick={onSignOut} disabled={busy} className="menu-item w-full text-left">
              {busy ? "Signing out…" : "Sign out"}
            </button>
          </li>
        </ul>
      </div>
    </div>
  );
}
