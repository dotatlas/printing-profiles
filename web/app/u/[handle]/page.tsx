import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { cache } from "react";
import { PresetList } from "../../../components/preset-list";
import { formatDate } from "../../../lib/format";
import { getProfile, isHandle, listPresets, profilePath } from "../../../lib/presets";

// The page waits for the API before it answers, so link-preview bots get a real 404 for an unknown handle.
export const instant = false;

// profileFor reads a profile once per request, or gives null for a path that is not a handle. The metadata and the
// page share it.
const profileFor = cache(async (handle: string) => (isHandle(handle) ? getProfile(handle) : null));

// generateMetadata names the member in the title.
export async function generateMetadata({ params }: PageProps<"/u/[handle]">): Promise<Metadata> {
  // The params first: they make the page a request-time page, so the build never calls the API here.
  const { handle } = await params;
  const profile = await profileFor(handle);
  if (!profile) return { title: "Member not found" };
  return {
    title: profile.display_name ? `${profile.display_name} (@${profile.handle})` : `@${profile.handle}`,
    description: `Slicer presets shared by @${profile.handle}.`,
    alternates: { canonical: profilePath(profile.handle) },
  };
}

// countLabel says a count with its noun, such as "1 preset" or "3 presets". A no-break space keeps them on one line.
function countLabel(n: number, noun: string): string {
  return `${n}\u00a0${noun}${n === 1 ? "" : "s"}`;
}

// ProfilePage shows a member's public profile and their published presets, newest first. Next.js renders it on the
// server for "/u/<handle>", the same for every visitor. "Next page" follows the API cursor.
export default async function ProfilePage({ params, searchParams }: PageProps<"/u/[handle]">) {
  const { handle } = await params;
  const cursor = (await searchParams).cursor;
  const after = typeof cursor === "string" ? cursor : undefined;
  const profile = await profileFor(handle);
  if (!profile) notFound();
  const result = await listPresets({ author: profile.handle, cursor: after });
  // Only a hand-made cursor gets a 400; the page then starts again from the newest.
  const page = result.ok ? result.page : { items: [], next_cursor: undefined };

  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight wrap-anywhere">{profile.display_name || `@${profile.handle}`}</h1>
      <p className="mt-2 text-muted wrap-anywhere">
        {profile.display_name && <>@{profile.handle} · </>}
        <span className="whitespace-nowrap">Joined {formatDate(profile.joined_at)}</span> · {countLabel(profile.preset_count, "preset")} ·{" "}
        {countLabel(profile.review_count, "review")}
      </p>
      <section aria-labelledby="presets" className="mt-8">
        <h2 id="presets" className="mb-3 text-sm font-medium">
          Presets
          {after && <span className="font-normal text-muted"> · later page</span>}
        </h2>
        {page.items.length > 0 ? (
          <PresetList items={page.items} showAuthor={false} />
        ) : (
          <p className="text-muted">{after ? "There are no more presets." : "No published presets yet."}</p>
        )}
      </section>
      {(after || page.next_cursor) && (
        <nav aria-label="Pages" className="mt-6 flex flex-wrap items-center gap-3">
          {after && <Link href={profilePath(profile.handle)}>Back to the newest</Link>}
          {page.next_cursor && (
            <Link
              href={`${profilePath(profile.handle)}?${new URLSearchParams({ cursor: page.next_cursor })}`}
              className="btn btn-secondary ml-auto"
            >
              Next page
            </Link>
          )}
        </nav>
      )}
    </>
  );
}
