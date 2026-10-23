import { apiURL } from "./api";
import type { Preset, PresetPage, Problem } from "./api-types";

// browseFilters are the filters of the browse page, in the order of its columns and of its URLs.
export const browseFilters = ["printer", "material", "nozzle", "slicer"] as const;

export type BrowseFilter = (typeof browseFilters)[number];
export type BrowseQuery = Partial<Record<BrowseFilter, string>>;

// presetPath gives the site's path of a preset. Links use it, and parsePresetParam reads it back.
export function presetPath(preset: { slug: string; id: string }): string {
  return `/presets/${preset.slug}-${preset.id}`;
}

// parsePresetParam splits the "<slug>-<id>" part of a preset path, or gives null when it holds no public ID.
// The ID uses a-z and 2-9 without l, o, 0 and 1, the same alphabet as newPublicID in the API.
export function parsePresetParam(param: string): { slug: string; id: string } | null {
  const match = /^(?:(.+)-)?([a-km-np-z2-9]{8})$/.exec(param);
  return match ? { slug: match[1] ?? "", id: match[2] } : null;
}

// cleanBrowseQuery keeps the browse filters and the cursor that have a value. clean is false when it dropped a part
// of the query, so the browse page can redirect to the short URL.
export function cleanBrowseQuery(params: Record<string, string | string[] | undefined>): {
  query: BrowseQuery;
  cursor?: string;
  clean: boolean;
} {
  const query: BrowseQuery = {};
  let cursor: string | undefined;
  let clean = true;
  for (const [key, raw] of Object.entries(params)) {
    const value = Array.isArray(raw) ? raw[0] : raw;
    if (!value || Array.isArray(raw)) clean = false;
    if (!value) continue;
    if (key === "cursor") cursor = value;
    else if ((browseFilters as readonly string[]).includes(key)) query[key as BrowseFilter] = value;
    else clean = false;
  }
  return { query, cursor, clean };
}

// browsePath gives the path of the browse page for a query, with an optional cursor for a later page.
export function browsePath(query: BrowseQuery, page?: { cursor: string }): string {
  const search = new URLSearchParams();
  for (const key of browseFilters) {
    const value = query[key];
    if (value) search.set(key, value);
  }
  if (page) search.set("cursor", page.cursor);
  const text = search.toString();
  return text ? `/presets?${text}` : "/presets";
}

// nozzleLabel writes a nozzle diameter like a slicer does: "0.4 mm", "0.25 mm", "1.0 mm".
export function nozzleLabel(mm: number): string {
  return `${Number.isInteger(mm * 10) ? mm.toFixed(1) : mm.toFixed(2)} mm`;
}

// printerLabel and materialLabel give the full names that the rows and the preset page show.
export function printerLabel(printer: Preset["printer"]): string {
  return `${printer.brand.name} ${printer.model}`;
}

export function materialLabel(material: Preset["material"]): string {
  return `${material.brand.name} ${material.product_name}`;
}

// getPreset gets one preset from the API on the server, without cookies, so it sees only published presets.
// It gives null when the API answers 404. The preset page calls it.
export async function getPreset(id: string): Promise<Preset | null> {
  const res = await fetch(new URL(`/api/v1/presets/${id}`, apiURL()), { signal: AbortSignal.timeout(10_000) });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`GET /api/v1/presets/{id}: status ${res.status}`);
  return (await res.json()) as Preset;
}

export type BrowseResult = { ok: true; page: PresetPage } | { ok: false; problem: string };

// listPresets gets one page of published presets for the browse page. A 400 (a filter that the API refuses)
// comes back as a message for the page; any other error fails.
export async function listPresets(query: BrowseQuery & { cursor?: string }): Promise<BrowseResult> {
  const search = new URLSearchParams();
  for (const key of [...browseFilters, "cursor"] as const) {
    const value = query[key];
    if (value) search.set(key, value);
  }
  search.set("limit", "20");
  const res = await fetch(new URL(`/api/v1/presets?${search}`, apiURL()), { signal: AbortSignal.timeout(10_000) });
  if (res.status === 400) {
    const problem = (await res.json()) as Problem;
    return { ok: false, problem: problem.detail ?? problem.title };
  }
  if (!res.ok) throw new Error(`GET /api/v1/presets: status ${res.status}`);
  return { ok: true, page: (await res.json()) as PresetPage };
}
