import type { Metadata } from "next";
import { notFound, permanentRedirect } from "next/navigation";
import { cache, Suspense } from "react";
import { AuthorActions, SavedNotice } from "../../../components/preset-pages";
import { BookmarkButton } from "../../../components/bookmark-button";
import { Comments } from "../../../components/comments";
import { Reviews } from "../../../components/reviews";
import { PresetView } from "../../../components/preset-view";
import { getComments, getPreset, getReviews, materialLabel, nozzleLabel, parsePresetParam, presetPath, printerLabel } from "../../../lib/presets";

// The page waits for the API before it answers. Link-preview bots then get a real 404 or 308; browsers get the page
// shell at once, then the not-found page (marked noindex) or the redirect in the browser.
export const instant = false;

// presetFor reads the preset of a "<slug>-<id>" path part once per request. The metadata and the page share it.
const presetFor = cache(async (param: string) => {
  const parsed = parsePresetParam(param);
  return parsed ? { parsed, preset: await getPreset(parsed.id) } : null;
});

// generateMetadata gives the title and a description made from the four facts. Next.js calls it with the page.
export async function generateMetadata({ params }: PageProps<"/presets/[slug]">): Promise<Metadata> {
  const found = await presetFor((await params).slug);
  if (!found?.preset) return {};
  const p = found.preset;
  return {
    title: p.title,
    description: `A ${materialLabel(p.material)} preset for the ${printerLabel(p.printer)} with a ${nozzleLabel(p.nozzle_mm)} nozzle, for ${p.slicer.name}.`,
    alternates: { canonical: presetPath(p) },
  };
}

// PresetPage shows one published preset. Next.js renders it on the server for "/presets/<slug>-<id>" without user
// data, so the page is the same for every visitor.
export default async function PresetPage({ params }: PageProps<"/presets/[slug]">) {
  const { slug } = await params;
  // The reviews and comments come in parallel with the preset; the ID in the path is enough to ask for them.
  const id = parsePresetParam(slug)?.id;
  const [found, reviews, comments] = await Promise.all([
    presetFor(slug),
    id ? getReviews(id) : null,
    id ? getComments(id) : null,
  ]);
  if (!found?.preset) notFound();
  const { parsed, preset } = found;
  // A link with an old or a missing slug goes to the current path, so each preset has one URL.
  if (parsed.slug !== preset.slug) permanentRedirect(presetPath(preset));

  return (
    <PresetView
      preset={preset}
      actions={
        <div className="flex flex-wrap items-start gap-3">
          <BookmarkButton presetId={preset.id} />
          <AuthorActions preset={preset} />
        </div>
      }
      community={
        <>
          <Reviews presetId={preset.id} authorHandle={preset.author.handle} initial={reviews?.items ?? []} next={reviews?.next_cursor} />
          <Comments presetId={preset.id} authorHandle={preset.author.handle} initial={comments?.items ?? []} next={comments?.next_cursor} />
        </>
      }
      // The notice reads the address, so it waits inside Suspense; the page HTML stays the same for everyone.
      notice={
        <Suspense>
          <SavedNotice />
        </Suspense>
      }
    />
  );
}
