import type { Metadata } from "next";
import { Suspense } from "react";
import { MyBookmarks } from "../../components/my-bookmarks";

export const metadata: Metadata = { title: "My bookmarks", robots: { index: false } };

// MyBookmarksPage is the route /my-bookmarks: the presets that the member bookmarked. The list is the member's own
// data, so it loads in the browser.
export default function MyBookmarksPage() {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">My bookmarks</h1>
      <Suspense>
        <MyBookmarks />
      </Suspense>
    </>
  );
}
