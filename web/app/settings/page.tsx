import type { Metadata } from "next";
import { Suspense } from "react";
import { Settings } from "../../components/settings";

export const metadata: Metadata = { title: "Settings", robots: { index: false } };

// SettingsPage is the account settings route. The heading is in the server HTML; the sections show the signed-in
// user's own data, so they render in the browser.
export default function SettingsPage() {
  return (
    // The frame is wide for the preset lists; forms read better at this width.
    <div className="max-w-3xl">
      <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
      <Suspense>
        <Settings />
      </Suspense>
    </div>
  );
}
