import { apiURL } from "./api";
import type { CommentPage, Preset, PresetPage, Problem, Profile, ReviewPage } from "./api-types";

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

// nozzleLabel writes a nozzle diameter like a slicer does: "0.4 mm", "0.25 mm", "1.0 mm". A no-break space keeps
// the number and the unit on one line.
export function nozzleLabel(mm: number): string {
  return `${Number.isInteger(mm * 10) ? mm.toFixed(1) : mm.toFixed(2)}\u00a0mm`;
}

// printerLabel and materialLabel give the full names that the rows and the preset page show.
export function printerLabel(printer: Preset["printer"]): string {
  return `${printer.brand.name} ${printer.model}`;
}

export function materialLabel(material: Preset["material"]): string {
  return `${material.brand.name} ${material.product_name}`;
}

// isHandle says whether a value can be a member's handle: the API's rule, 3 to 30 characters. The profile page
// checks the path with it before it asks the API.
export function isHandle(value: string): boolean {
  return value.length >= 3 && value.length <= 30 && /^[a-z0-9]+(-[a-z0-9]+)*$/.test(value);
}

// profilePath gives the path of a member's profile page. The bylines of presets, reviews and comments link to it.
export function profilePath(handle: string): string {
  return `/u/${handle}`;
}

// authorName names the author of a preset in a byline: "Mia Hartmann (@mia)", or "@mia" for a user without a
// display name. The preset page uses it; the rows show the shorter display name alone.
export function authorName(author: Preset["author"]): string {
  return author.display_name ? `${author.display_name} (@${author.handle})` : `@${author.handle}`;
}

// getPublic reads public JSON from the API on the server, without cookies, or gives null when the API answers 404.
// getPreset, the review and comment reads and getProfile use it.
async function getPublic<T>(path: string, label: string): Promise<T | null> {
  const res = await fetch(new URL(path, apiURL()), { signal: AbortSignal.timeout(10_000) });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`GET ${label}: status ${res.status}`);
  return (await res.json()) as T;
}

// getPreset gets one published preset, or null. The preset page calls it; the server never sees drafts.
export function getPreset(id: string): Promise<Preset | null> {
  return getPublic<Preset>(`/api/v1/presets/${id}`, "/api/v1/presets/{id}");
}

// getReviews and getComments get the first page of a published preset's reviews and comments for the preset page.
export function getReviews(id: string): Promise<ReviewPage | null> {
  return getPublic<ReviewPage>(`/api/v1/presets/${id}/reviews`, "/api/v1/presets/{id}/reviews");
}

export function getComments(id: string): Promise<CommentPage | null> {
  return getPublic<CommentPage>(`/api/v1/presets/${id}/comments`, "/api/v1/presets/{id}/comments");
}

// getProfile gets a member's public profile, or null for an unknown handle. The profile page calls it.
export function getProfile(handle: string): Promise<Profile | null> {
  return getPublic<Profile>(`/api/v1/users/${encodeURIComponent(handle)}`, "/api/v1/users/{handle}");
}

export type BrowseResult = { ok: true; page: PresetPage } | { ok: false; problem: string };

// listPresets gets one page of published presets for the browse page, or one author's presets for a profile page.
// A 400 (a filter that the API refuses) comes back as a message for the page; any other error fails.
export async function listPresets(query: BrowseQuery & { cursor?: string; author?: string }): Promise<BrowseResult> {
  const search = new URLSearchParams();
  for (const key of [...browseFilters, "cursor", "author"] as const) {
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
