import type { Metadata } from "next";
import { Suspense } from "react";
import { Welcome } from "../../components/welcome";

export const metadata: Metadata = { title: "Choose your handle", robots: { index: false } };

// WelcomePage is the first-sign-in step. The heading is in the server HTML; the form needs the signed-in user, so
// it renders in the browser.
export default function WelcomePage() {
  return (
    <div className="mx-auto max-w-sm">
      <h1 className="text-2xl font-semibold tracking-tight">Choose your handle</h1>
      <Suspense>
        <Welcome />
      </Suspense>
    </div>
  );
}
