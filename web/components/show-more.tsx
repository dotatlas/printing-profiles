"use client";

import { useState } from "react";

// ShowMore is the button under a list that reads the next page, as for reviews and comments. It ignores a second
// press while a page loads, and says when the page did not load.
export function ShowMore({ label, onMore, failed }: { label: string; onMore: () => Promise<void>; failed: boolean }) {
  const [busy, setBusy] = useState(false);

  async function onClick() {
    if (busy) return;
    setBusy(true);
    try {
      await onMore();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mt-6 flex flex-col items-start gap-2">
      <button type="button" className="btn btn-secondary" onClick={onClick} aria-busy={busy || undefined}>
        {label}
      </button>
      {failed && (
        <p role="alert" className="text-sm text-danger">
          More did not load. Try again.
        </p>
      )}
    </div>
  );
}
