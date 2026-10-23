import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { MyPresets } from "../../components/preset-pages";

export const metadata: Metadata = { title: "My presets", robots: { index: false } };

// MyPresetsPage is the route /my-presets: the member's drafts and published presets. The list is the member's own
// data, so it loads in the browser.
export default function MyPresetsPage() {
  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold tracking-tight">My presets</h1>
        <Link href="/presets/new" className="btn btn-primary">
          Share a preset
        </Link>
      </div>
      <Suspense>
        <MyPresets />
      </Suspense>
    </>
  );
}
