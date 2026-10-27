import Link from "next/link";
import type { ReactNode } from "react";
import { type Material, materialTypeValues, nozzleValues, type Preset, type Printer, type Slicer } from "../lib/api-types";
import { formatDate } from "../lib/format";
import {
  type BrowseFilter,
  type BrowseQuery,
  browseFilters,
  browsePath,
  materialLabel,
  nozzleLabel,
  presetPath,
  printerLabel,
} from "../lib/presets";
import { RatingInline } from "./stars";

export type Catalog = { printers: Printer[]; materials: Material[]; slicers: Slicer[] };

// columns is the one grid of the browse page. The filters and the rows share it, so each filter sits on top of the
// column that it filters, and its label is that column's heading. From lg (992px of text width at most); a narrower
// grid cuts the names in the selects.
const columns =
  "lg:grid lg:grid-cols-[minmax(0,2.4fr)_minmax(0,1.4fr)_minmax(0,1.6fr)_minmax(0,0.8fr)_minmax(0,1.2fr)] lg:gap-x-4";

const filterNames: Record<BrowseFilter, string> = { printer: "Printer", material: "Filament", nozzle: "Nozzle", slicer: "Slicer" };

// filterLabel gives the name that the page shows for a filter value, or the value itself when the catalog does not
// know it. NoMatches and setupLabel use it.
function filterLabel(catalog: Catalog, key: BrowseFilter, value: string): string {
  if (key === "printer") {
    const printer = catalog.printers.find((p) => p.slug === value);
    return printer ? printerLabel(printer) : value;
  }
  if (key === "material") {
    const material = catalog.materials.find((m) => m.slug === value);
    return material ? materialLabel(material) : value;
  }
  if (key === "slicer") return catalog.slicers.find((s) => s.slug === value)?.name ?? value;
  const mm = Number(value);
  return (nozzleValues as readonly number[]).includes(mm) ? nozzleLabel(mm) : value;
}

// setupLabel names the filters in use, such as "Bambu Lab P1S, 0.4 mm", or gives "" without filters. The browse
// page uses it for its title and its results heading.
export function setupLabel(catalog: Catalog, query: BrowseQuery): string {
  return browseFilters
    .filter((key) => query[key])
    .map((key) => filterLabel(catalog, key, query[key] ?? ""))
    .join(", ");
}

// PrinterOptions and MaterialOptions are the select options of the catalog, grouped by brand and by filament type.
// The browse filters and the preset form use them.
export function PrinterOptions({ printers }: { printers: Printer[] }) {
  return [...Map.groupBy(printers, (p) => p.brand.name)].map(([brand, items]) => (
    <optgroup key={brand} label={brand}>
      {items.map((p) => (
        <option key={p.slug} value={p.slug}>
          {printerLabel(p)}
        </option>
      ))}
    </optgroup>
  ));
}

export function MaterialOptions({ materials }: { materials: Material[] }) {
  const byType = Map.groupBy(materials, (m) => m.type);
  // The types come in the order of the API contract, so the most common type (PLA) is first.
  return materialTypeValues.map((type) => {
    const items = byType.get(type);
    return (
      items && (
        <optgroup key={type} label={type}>
          {items.map((m) => (
            <option key={m.slug} value={m.slug}>
              {materialLabel(m)}
            </option>
          ))}
        </optgroup>
      )
    );
  });
}

// FilterSelect is one filter: its label, which is also the column heading, and a native select. A select in use gets
// the accent tint, so the filters in use show at a glance.
function FilterSelect({
  name,
  value,
  known,
  children,
}: {
  name: BrowseFilter;
  value?: string;
  known: boolean;
  children: ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={`filter-${name}`} className="text-[13px] font-medium text-muted">
        {filterNames[name]}
      </label>
      <select id={`filter-${name}`} name={name} defaultValue={value ?? ""} data-active={Boolean(value)} className="field filter-select">
        <option value="">Any</option>
        {/* A value from an old link stays visible, so the select never claims "Any" for a filter in use. */}
        {value && !known && <option value={value}>{value}</option>}
        {children}
      </select>
    </div>
  );
}

// BrowseFilters is the filter form of the browse page. It is a plain GET form, so it works without JavaScript.
export function BrowseFilters({ catalog, query }: { catalog: Catalog; query: BrowseQuery }) {
  const inUse = browseFilters.some((key) => query[key]);
  return (
    <form action="/presets" method="get" role="search" aria-label="Filter presets">
      <div className={`grid grid-cols-2 gap-3 md:grid-cols-4 ${columns}`}>
        {/* The heading of the title column. The cell under it stays empty, so no text looks like a control. */}
        <div className="hidden lg:block">
          <span className="text-[13px] font-medium text-muted">Preset</span>
        </div>
        <FilterSelect name="printer" value={query.printer} known={catalog.printers.some((p) => p.slug === query.printer)}>
          <PrinterOptions printers={catalog.printers} />
        </FilterSelect>
        <FilterSelect name="material" value={query.material} known={catalog.materials.some((m) => m.slug === query.material)}>
          <MaterialOptions materials={catalog.materials} />
        </FilterSelect>
        <FilterSelect name="nozzle" value={query.nozzle} known={(nozzleValues as readonly number[]).includes(Number(query.nozzle))}>
          {nozzleValues.map((mm) => (
            <option key={mm} value={String(mm)}>
              {nozzleLabel(mm)}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect name="slicer" value={query.slicer} known={catalog.slicers.some((s) => s.slug === query.slicer)}>
          {catalog.slicers.map((s) => (
            <option key={s.slug} value={s.slug}>
              {s.name}
            </option>
          ))}
        </FilterSelect>
      </div>
      <div className="mt-3 flex flex-wrap justify-end gap-3">
        {inUse && (
          <Link href="/presets" className="btn btn-secondary">
            Clear filters
          </Link>
        )}
        <button type="submit" className="btn btn-primary max-sm:flex-1">
          Show presets
        </button>
      </div>
    </form>
  );
}

// PresetList shows one page of presets as rows on the grid of the filters. The whole row is the link to the preset.
// Below lg, the four facts sit under the title in two columns. A fact that a filter already fixes is muted, so the
// facts that differ stand out. A profile page sets showAuthor to false, because the page names the author.
export function PresetList({ items, query = {}, showAuthor = true }: { items: Preset[]; query?: BrowseQuery; showAuthor?: boolean }) {
  return (
    <ul className="divide-y divide-line border-y border-line">
      {items.map((preset) => (
        <li key={preset.id} className={`group relative py-3 lg:items-baseline ${columns}`}>
          <div className="min-w-0">
            {/* The link covers the whole row, so the row is one large target on a phone. */}
            <Link
              href={presetPath(preset)}
              className="font-medium text-ink no-underline wrap-anywhere after:absolute after:inset-0 group-hover:underline"
            >
              {preset.title}
            </Link>
            <p className="mt-0.5 text-[13px] text-muted wrap-anywhere">
              {showAuthor && <>by {preset.author.display_name || `@${preset.author.handle}`} · </>}
              {formatDate(preset.published_at ?? preset.created_at)}
              {preset.rating && (
                <>
                  {" · "}
                  <RatingInline rating={preset.rating} />
                </>
              )}
            </p>
          </div>
          <dl className="mt-1.5 grid grid-cols-2 gap-x-3 gap-y-0.5 text-[13px] md:grid-cols-4 lg:contents lg:text-sm">
            <Fact label="Printer" fixed={query.printer === preset.printer.slug}>
              {printerLabel(preset.printer)}
            </Fact>
            <Fact label="Filament" fixed={query.material === preset.material.slug}>
              {materialLabel(preset.material)}
            </Fact>
            <Fact label="Nozzle" fixed={Number(query.nozzle) === preset.nozzle_mm}>
              <span className="tabular-nums">{nozzleLabel(preset.nozzle_mm)}</span>
            </Fact>
            <Fact label="Slicer" fixed={query.slicer === preset.slicer.slug}>
              {preset.slicer.name}
              {preset.slicer_version && <span className="text-muted"> {preset.slicer_version}</span>}
            </Fact>
          </dl>
        </li>
      ))}
    </ul>
  );
}

// Fact is one fact of a row: a label for screen readers and the value, muted when a filter fixes it.
function Fact({ label, fixed, children }: { label: string; fixed: boolean; children: ReactNode }) {
  return (
    <div className={`min-w-0 wrap-anywhere ${fixed ? "text-muted" : "text-ink"}`}>
      <dt className="sr-only">{label}</dt>
      <dd className="inline">{children}</dd>
    </div>
  );
}

// NoMatches is the empty state for filters that match no preset. It offers to remove one filter at a time, because
// one filter too many is the usual cause.
export function NoMatches({ catalog, query }: { catalog: Catalog; query: BrowseQuery }) {
  const inUse = browseFilters.filter((key) => query[key]);
  return (
    <div className="panel">
      <h3 className="font-semibold">No presets match these filters</h3>
      <p className="mt-1 text-sm text-muted">Remove a filter to see more presets.</p>
      <ul className="mt-4 flex flex-wrap gap-2">
        {inUse.map((key) => (
          <li key={key}>
            {/* A label from an old link can be long, so this button wraps instead of running off the screen. */}
            <Link
              href={browsePath({ ...query, [key]: undefined })}
              className="btn btn-secondary h-auto min-h-10 py-2 text-left whitespace-normal wrap-anywhere"
            >
              Remove {filterLabel(catalog, key, query[key] ?? "")}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
