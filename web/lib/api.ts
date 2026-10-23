import type { paths } from "./api-types";

// ListPath is a catalog list route of the API.
type ListPath = "/api/v1/printers" | "/api/v1/materials" | "/api/v1/slicers";

// Items is the item type of a list route, from the API contract.
type Items<P extends ListPath> = paths[P]["get"]["responses"][200]["content"]["application/json"]["items"];

// apiURL gives the base URL of the API for the server reads (listFirst, lib/presets). In production, API_URL must be set.
export function apiURL(): string {
  const url = process.env.API_URL;
  if (url) return url;
  if (process.env.NODE_ENV === "production") throw new Error("API_URL must be set in production");
  return "http://localhost:8080";
}

// listFirst gets the first page of a catalog list from the API. The home page calls it on the server, without cookies.
export async function listFirst<P extends ListPath>(path: P): Promise<Items<P>> {
  // shortcut: one page of at most 100 items fits the catalog now; follow next_cursor when a list grows past 100.
  const res = await fetch(new URL(`${path}?limit=100`, apiURL()), { signal: AbortSignal.timeout(10_000) });
  if (!res.ok) throw new Error(`GET ${path}: status ${res.status}`);
  // The API contract gives the shape of the page, and the API tests check it.
  const page: { items: Items<P> } = await res.json();
  return page.items;
}
