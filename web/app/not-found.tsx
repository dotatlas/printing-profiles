import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Page not found" };

// NotFound shows for a URL that the site does not have. Next.js sends it with status 404.
export default function NotFound() {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Page not found</h1>
      <p className="mt-2 text-muted">The page that you asked for does not exist.</p>
      <p className="mt-6">
        <Link href="/">Go to the home page</Link>
      </p>
    </>
  );
}
