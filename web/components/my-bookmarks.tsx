"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import type { Preset } from "../lib/api-types";
import { listBookmarks } from "../lib/community";
import { Notice } from "./account-ui";
import { PresetList } from "./preset-list";
import { Loading } from "./preset-pages";
import { useMember } from "./session";

// MyBookmarks is the body of /my-bookmarks: the member's bookmarked presets, newest bookmark first, as the browse
// rows. The list is the member's own data, so it loads in the browser.
export function MyBookmarks() {
  const me = useMember("/my-bookmarks");
  const [loaded, setLoaded] = useState<{ handle: string; items: Preset[] } | "failed" | null>(null);
  const handle = me?.handle ?? null;
  useEffect(() => {
    if (!handle) return;
    let live = true;
    listBookmarks().then(
      (page) => live && setLoaded({ handle, items: page.items }),
      () => live && setLoaded("failed"),
    );
    return () => {
      live = false;
    };
  }, [handle]);

  if (!me || loaded === null || (loaded !== "failed" && loaded.handle !== handle)) return <Loading />;
  if (loaded === "failed") {
    return (
      <div className="mt-6">
        <Notice tone="error">Your bookmarks did not load. Reload the page to try again.</Notice>
      </div>
    );
  }
  if (loaded.items.length === 0) {
    return (
      <div className="panel mt-6">
        <h2 className="font-semibold">No bookmarks yet</h2>
        <p className="mt-1 text-sm text-muted">Press Bookmark on a preset to keep it here for your next print.</p>
        <Link href="/presets" className="btn btn-primary mt-4">
          Browse presets
        </Link>
      </div>
    );
  }
  return (
    <div className="mt-6">
      <PresetList items={loaded.items} />
    </div>
  );
}
