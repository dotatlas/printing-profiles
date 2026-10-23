import Link from "next/link";
import type { License, Preset } from "../lib/api-types";
import { formatDate } from "../lib/format";
import { browsePath, materialLabel, nozzleLabel, printerLabel } from "../lib/presets";

// licenses gives the short name and the text URL of each license that a preset can have.
const licenses: Record<License, { name: string; url: string }> = {
  "CC-BY-4.0": { name: "CC BY 4.0", url: "https://creativecommons.org/licenses/by/4.0/" },
  "CC-BY-SA-4.0": { name: "CC BY-SA 4.0", url: "https://creativecommons.org/licenses/by-sa/4.0/" },
  "CC-BY-NC-4.0": { name: "CC BY-NC 4.0", url: "https://creativecommons.org/licenses/by-nc/4.0/" },
  "CC0-1.0": { name: "CC0 1.0", url: "https://creativecommons.org/publicdomain/zero/1.0/" },
};

// PresetFacts shows the four facts of a preset as tags, each with its kind. Each tag opens the browse page with that
// one filter, and the link under them opens it with all four, so a visitor goes from one preset to every preset for
// the same setup in one step. PresetView uses it.
export function PresetFacts({ preset }: { preset: Preset }) {
  const slicer = preset.slicer_version ? `${preset.slicer.name} ${preset.slicer_version}` : preset.slicer.name;
  const facts = [
    { key: "printer", kind: "Printer", value: preset.printer.slug, text: printerLabel(preset.printer) },
    { key: "material", kind: "Filament", value: preset.material.slug, text: materialLabel(preset.material) },
    { key: "nozzle", kind: "Nozzle", value: String(preset.nozzle_mm), text: nozzleLabel(preset.nozzle_mm) },
    { key: "slicer", kind: "Slicer", value: preset.slicer.slug, text: slicer },
  ] as const;
  const setup = {
    printer: preset.printer.slug,
    material: preset.material.slug,
    nozzle: String(preset.nozzle_mm),
    slicer: preset.slicer.slug,
  };
  return (
    <>
      <ul className="flex flex-wrap gap-2">
        {facts.map((fact) => (
          <li key={fact.key} className="min-w-0">
            <Link href={browsePath({ [fact.key]: fact.value })} className="fact-tag">
              {/* The spaces between the parts are for screen readers; a flex box does not show them. */}
              <span className="sr-only">More presets for the</span>{" "}
              <span className="fact-kind">{fact.kind}</span>{" "}
              <span className="wrap-anywhere">{fact.text}</span>
            </Link>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-sm">
        <Link href={browsePath(setup)}>See every preset for this setup</Link>
      </p>
    </>
  );
}

// PresetDetails lists the license, the edit date and the file of a preset. PresetView shows it beside the
// description; the byline and the tags already give the author, the date and the slicer.
export function PresetDetails({ preset }: { preset: Preset }) {
  const license = licenses[preset.license];
  const published = preset.published_at ?? preset.created_at;
  const edited = formatDate(preset.updated_at) !== formatDate(published);
  return (
    <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
      <dt className="text-muted">License</dt>
      <dd>
        <a href={license.url} rel="license">
          {license.name}
        </a>
      </dd>
      {edited && (
        <>
          <dt className="text-muted">Edited</dt>
          <dd>{formatDate(preset.updated_at)}</dd>
        </>
      )}
      <dt className="text-muted">File</dt>
      <dd className="text-muted">None attached yet</dd>
    </dl>
  );
}
