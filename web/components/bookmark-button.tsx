"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { welcomePath } from "../lib/account";
import { addBookmark, isBookmarked, removeBookmark } from "../lib/community";
import { useSession } from "./session";

// BookmarkMark is the bookmark icon of the button, filled when the preset is bookmarked.
function BookmarkMark({ filled }: { filled: boolean }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-4">
      <path
        d="M4 2.5h8a.5.5 0 0 1 .5.5v10.6a.3.3 0 0 1-.48.24L8 10.9l-4.02 2.94a.3.3 0 0 1-.48-.24V3a.5.5 0 0 1 .5-.5Z"
        fill={filled ? "currentColor" : "none"}
        stroke="currentColor"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
    </svg>
  );
}

// BookmarkButton adds a preset to the member's bookmarks or takes it out, on the preset page. A visitor gets a link to
// sign in. The page is the same HTML for everyone, so the state loads in the browser.
export function BookmarkButton({ presetId }: { presetId: string }) {
  const { state } = useSession();
  const pathname = usePathname();
  const [saved, setSaved] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const member = state.status === "signed-in" && !state.me.needs_onboarding;

  useEffect(() => {
    if (!member) return;
    let live = true;
    isBookmarked(presetId).then(
      (b) => live && setSaved(b),
      () => live && setSaved(false),
    );
    return () => {
      live = false;
    };
  }, [member, presetId]);

  if (state.status === "signed-out") {
    return (
      <Link href={`/signin?return=${encodeURIComponent(pathname)}`} className="btn btn-secondary">
        <BookmarkMark filled={false} />
        Bookmark
      </Link>
    );
  }
  if (state.status === "signed-in" && state.me.needs_onboarding) {
    return (
      <Link href={welcomePath(pathname)} className="btn btn-secondary">
        <BookmarkMark filled={false} />
        Bookmark
      </Link>
    );
  }
  // The button's space stays while the state loads, so the title row does not jump.
  if (saved === null) return <div className="h-10 w-32" aria-hidden="true" />;

  async function onToggle() {
    if (busy) return;
    const next = !saved;
    // The button changes at once; it goes back if the API refuses.
    setSaved(next);
    setBusy(true);
    setFailed(false);
    try {
      await (next ? addBookmark(presetId) : removeBookmark(presetId));
    } catch {
      setSaved(!next);
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-start gap-1">
      <button type="button" className="btn btn-secondary btn-toggle" aria-pressed={saved} onClick={onToggle}>
        <BookmarkMark filled={saved} />
        Bookmark
      </button>
      {failed && (
        <p role="alert" className="text-sm text-danger">
          The bookmark did not change. Try again.
        </p>
      )}
    </div>
  );
}
