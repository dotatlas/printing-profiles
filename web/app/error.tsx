"use client";

// ErrorPage shows in place of a page that fails, for example when the API does not answer. Next.js calls it.
export default function ErrorPage({ error, retry }: { error: Error & { digest?: string }; retry: () => void }) {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Something went wrong</h1>
      <p className="mt-2 text-muted">This page could not load. Try again in a moment.</p>
      <button
        type="button"
        onClick={() => retry()}
        className="mt-6 rounded-md bg-accent px-4 py-2 font-medium text-page hover:opacity-90"
      >
        Try again
      </button>
      {/* The digest matches the server log line. The message itself can hold internal details, so it stays hidden. */}
      {error.digest && <p className="mt-6 text-sm text-muted">Error ID: {error.digest}</p>}
    </>
  );
}
