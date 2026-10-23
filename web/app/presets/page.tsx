import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { cache } from "react";
import { Notice } from "../../components/account-ui";
import { BrowseFilters, NoMatches, PresetList, setupLabel } from "../../components/preset-list";
import { listFirst } from "../../lib/api";
import { browseFilters, browsePath, cleanBrowseQuery, listPresets } from "../../lib/presets";

// The page waits for the API before it answers. Link-preview bots then get real redirects; browsers get the page
// shell at once, and the redirect follows in the browser.
export const instant = false;

// catalog reads the three catalog lists once per request. The metadata and the page share it.
const catalog = cache(async () => {
  const [printers, materials, slicers] = await Promise.all([
    listFirst("/api/v1/printers"),
    listFirst("/api/v1/materials"),
    listFirst("/api/v1/slicers"),
  ]);
  return { printers, materials, slicers };
});

// generateMetadata names the filters in the title, so the tab and a shared link say which presets they show.
export async function generateMetadata({ searchParams }: PageProps<"/presets">): Promise<Metadata> {
  const setup = setupLabel(await catalog(), cleanBrowseQuery(await searchParams).query);
  return {
    title: setup ? `Presets for ${setup}` : "Browse presets",
    description: "Slicer presets for an exact printer, filament, nozzle and slicer, newest first.",
  };
}

// BrowsePage lists the published presets with the filters on top. Next.js renders it on the server for "/presets".
// The filters are a plain GET form, and "Next page" follows the API cursor.
export default async function BrowsePage({ searchParams }: PageProps<"/presets">) {
  const { query, cursor, clean } = cleanBrowseQuery(await searchParams);
  if (!clean) redirect(browsePath(query, cursor ? { cursor } : undefined));

  const [lists, result] = await Promise.all([catalog(), listPresets({ ...query, cursor })]);
  const filtered = browseFilters.some((key) => query[key]);
  const setup = setupLabel(lists, query);

  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Presets</h1>
      <p className="mt-2 text-muted">Pick your setup to see the presets made for it.</p>
      <div className="mt-8">
        <BrowseFilters catalog={lists} query={query} />
      </div>
      <section aria-labelledby="results" className="mt-8">
        <h2 id="results" className="mb-3 text-sm font-medium wrap-anywhere">
          {setup ? `Newest presets for ${setup}` : "Newest presets"}
          {cursor && <span className="font-normal text-muted"> · later page</span>}
        </h2>
        {!result.ok ? (
          // Only a hand-made link can get here: the selects send only values that the API knows.
          <Notice tone="error">A filter in this link is not one the site knows. Choose the filters again above.</Notice>
        ) : result.page.items.length > 0 ? (
          <PresetList items={result.page.items} query={query} />
        ) : cursor ? (
          <p className="text-muted">There are no more presets.</p>
        ) : filtered ? (
          <NoMatches catalog={lists} query={query} />
        ) : (
          <div className="panel">
            <p className="font-semibold">No presets yet</p>
            <p className="mt-1 text-sm text-muted">Nobody has shared a preset yet.</p>
          </div>
        )}
      </section>
      {result.ok && (cursor || result.page.next_cursor) && (
        <nav aria-label="Pages" className="mt-6 flex flex-wrap items-center gap-3">
          {cursor && <Link href={browsePath(query)}>Back to the newest</Link>}
          {result.page.next_cursor && (
            <Link href={browsePath(query, { cursor: result.page.next_cursor })} className="btn btn-secondary ml-auto">
              Next page
            </Link>
          )}
        </nav>
      )}
    </>
  );
}
