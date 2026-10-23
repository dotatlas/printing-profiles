import type { Metadata } from "next";
import { Suspense } from "react";
import { SignIn } from "../../components/sign-in";

export const metadata: Metadata = { title: "Sign in", robots: { index: false } };

// SignInPage is the sign-in route. The heading is in the server HTML; the form reads the return path from the URL,
// so it renders in the browser.
export default function SignInPage() {
  return (
    <div className="mx-auto max-w-sm">
      <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
      <Suspense>
        <SignIn />
      </Suspense>
    </div>
  );
}
