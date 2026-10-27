import Link from "next/link";
import type { ReactNode } from "react";
import type { Preset } from "../lib/api-types";
import { formatDate, tidyText } from "../lib/format";
import { authorName, profilePath } from "../lib/presets";
import { fileExt, fileKind, KeySettings, PresetDetails, PresetFacts, slicerName } from "./preset-facts";
import { RatingSummary } from "./stars";

// PresetView shows one preset: the title, the author, the rating, the facts as tags, the description and the
// details. The preset page renders it on the server; the draft view renders it in the browser. actions holds the
// author's buttons, notice a line above the title, and community the reviews and comments under the description.
export function PresetView({
  preset,
  actions,
  notice,
  community,
}: {
  preset: Preset;
  actions?: ReactNode;
  notice?: ReactNode;
  community?: ReactNode;
}) {
  return (
    <article>
      {/* empty:hidden drops the space when the notice renders nothing, as SavedNotice does without a save. */}
      {notice && <div className="mb-6 empty:hidden">{notice}</div>}
      <header className="flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
        <div className="min-w-0 flex-1 basis-80">
          <h1 className="text-2xl font-semibold tracking-tight text-balance wrap-anywhere sm:text-3xl">{preset.title}</h1>
          <p className="mt-2 text-muted wrap-anywhere">
            by{" "}
            <Link href={profilePath(preset.author.handle)} className="text-ink no-underline hover:underline">
              {authorName(preset.author)}
            </Link>{" "}
            ·{" "}
            {formatDate(preset.published_at ?? preset.created_at)}
          </p>
          {preset.status === "published" && (
            <div className="mt-2">
              <RatingSummary rating={preset.rating} />
            </div>
          )}
        </div>
        {actions}
      </header>
      <div className="mt-5">
        <PresetFacts preset={preset} />
      </div>
      <div className="mt-10 grid gap-8 md:grid-cols-[minmax(0,1fr)_16rem]">
        {/* The panel comes first in the page, so Download is near the top for the keyboard and on a phone. From md it
            moves to the second column and stays in view beside a long description. */}
        <aside aria-label="Details" className="panel flex flex-col gap-4 self-start md:sticky md:top-6 md:order-last">
          {preset.file && (
            <div className="flex flex-col gap-2">
              {/* A draft is not public, so its author gets the file itself, and nothing is counted. */}
              <a href={preset.status === "published" ? preset.file.download_url : preset.file.file_url} className="btn btn-primary w-full">
                Download {fileKind(preset.file.kind)} settings ({fileExt(preset.file.slicer)})
              </a>
              <p className="text-sm text-muted">For {slicerName(preset.file.slicer)}. Import it in the slicer.</p>
            </div>
          )}
          <PresetDetails preset={preset} />
        </aside>
        <div className="flex min-w-0 flex-col gap-10">
          {preset.file && (
            <section aria-labelledby="key-settings">
              <h2 id="key-settings" className="font-semibold">
                {fileKind(preset.file.kind).replace(/^./, (c) => c.toUpperCase())} settings in the file
              </h2>
              <p className="mt-1 text-sm text-muted">
                The key ones, read from the {slicerName(preset.file.slicer)}
                {preset.file.slicer_version && ` ${preset.file.slicer_version}`} file. It holds every other setting too.
              </p>
              <div className="mt-3">
                <KeySettings settings={preset.file.settings} />
              </div>
            </section>
          )}
          <section aria-labelledby="about">
            <h2 id="about" className="font-semibold">
              About this preset
            </h2>
            {preset.description ? (
              // Plain text: React escapes it, and pre-line keeps the author's line breaks.
              <p className="mt-2 max-w-prose whitespace-pre-line wrap-anywhere">{tidyText(preset.description)}</p>
            ) : (
              <p className="mt-2 text-muted">The author did not write a description.</p>
            )}
          </section>
          {community}
        </div>
      </div>
    </article>
  );
}
