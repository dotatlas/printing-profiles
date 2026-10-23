import Link from "next/link";
import type { KeySetting, License, Preset } from "../lib/api-types";
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

// PresetDetails lists the license, the edit date, and the download count or the missing file of a preset. PresetView shows it beside the
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
      {preset.file ? (
        <>
          <dt className="text-muted">Downloads</dt>
          <dd className="tabular-nums">{preset.download_count}</dd>
        </>
      ) : (
        <>
          <dt className="text-muted">File</dt>
          <dd className="text-muted">None attached yet</dd>
        </>
      )}
    </dl>
  );
}

const slicerNames: Record<string, string> = { prusaslicer: "PrusaSlicer", superslicer: "SuperSlicer", orcaslicer: "OrcaSlicer" };
const kindNames: Record<string, string> = { print: "print", filament: "filament", printer: "printer", config: "full" };

// fileLabel names a checked preset file, such as "PrusaSlicer filament settings, version 2.9.6". The form and the
// preset page use it.
export function fileLabel(file: { slicer: string; kind: string; slicer_version?: string }): string {
  const name = `${slicerNames[file.slicer] ?? file.slicer} ${kindNames[file.kind] ?? file.kind} settings`;
  return file.slicer_version ? `${name}, version ${file.slicer_version}` : name;
}

// KeySettings shows the key settings that the check read from a preset file, as a two-column table. The preset
// page shows it for visitors, and the form shows it right after a file is checked.
export function KeySettings({ settings }: { settings: KeySetting[] }) {
  if (settings.length === 0) return <p className="text-sm text-muted">The file holds none of the settings in this table.</p>;
  return (
    <dl className="grid grid-cols-[minmax(0,1fr)_auto] text-sm sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)]">
      {settings.map((s) => (
        <div key={s.id} className="contents">
          {/* Padding, not a column gap, so each row's rule runs unbroken across both columns. */}
          <dt className="border-b border-line py-1.5 pr-6 text-muted">{s.label}</dt>
          <dd className="border-b border-line py-1.5 tabular-nums wrap-anywhere">{s.unit ? `${s.value} ${s.unit}` : s.value}</dd>
        </div>
      ))}
    </dl>
  );
}
