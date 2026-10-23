import type { ReactNode } from "react";
import type { Preset } from "../lib/api-types";
import { formatDate } from "../lib/format";
import { authorName } from "../lib/presets";
import { PresetDetails, PresetFacts } from "./preset-facts";

// PresetView shows one preset: the title, the author, the facts as tags, the description and the details. The
// preset page renders it on the server; the draft view renders it in the browser. actions holds the author's
// buttons, and notice a line above the title.
export function PresetView({ preset, actions, notice }: { preset: Preset; actions?: ReactNode; notice?: ReactNode }) {
  return (
    <article>
      {/* empty:hidden drops the space when the notice renders nothing, as SavedNotice does without a save. */}
      {notice && <div className="mb-6 empty:hidden">{notice}</div>}
      <header className="flex flex-wrap items-start justify-between gap-x-6 gap-y-4">
        <div className="min-w-0 flex-1 basis-80">
          <h1 className="text-2xl font-semibold tracking-tight text-balance wrap-anywhere sm:text-3xl">{preset.title}</h1>
          <p className="mt-2 text-muted wrap-anywhere">
            by <span className="text-ink">{authorName(preset.author)}</span> ·{" "}
            {formatDate(preset.published_at ?? preset.created_at)}
          </p>
        </div>
        {actions}
      </header>
      <div className="mt-5">
        <PresetFacts preset={preset} />
      </div>
      <div className="mt-10 grid gap-8 md:grid-cols-[minmax(0,1fr)_16rem]">
        <section aria-labelledby="about">
          <h2 id="about" className="font-semibold">
            About this preset
          </h2>
          {preset.description ? (
            // Plain text: React escapes it, and pre-line keeps the author's line breaks.
            <p className="mt-2 max-w-prose whitespace-pre-line wrap-anywhere">{preset.description}</p>
          ) : (
            <p className="mt-2 text-muted">The author did not write a description.</p>
          )}
        </section>
        {/* The panel stays in view beside a long description. */}
        <aside aria-label="Details" className="panel self-start md:sticky md:top-6">
          <PresetDetails preset={preset} />
        </aside>
      </div>
    </article>
  );
}
