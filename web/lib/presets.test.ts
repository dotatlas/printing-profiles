// @vitest-environment node
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { afterAll, afterEach, beforeEach, expect, test, vi } from "vitest";
import {
  authorName,
  browsePath,
  cleanBrowseQuery,
  getPreset,
  isHandle,
  listPresets,
  nozzleLabel,
  parsePresetParam,
  presetPath,
  profilePath,
} from "./presets";

// The fake API knows one preset (abcdefgh), refuses the nozzle 0.7, and fails for the preset zzzzzzzz.
const requests: string[] = [];
const api = createServer((req, res) => {
  const url = req.url ?? "";
  requests.push(url);
  const send = (status: number, body: unknown) => {
    res.writeHead(status, { "Content-Type": status < 300 ? "application/json" : "application/problem+json" });
    res.end(JSON.stringify(body));
  };
  if (url === "/api/v1/presets/abcdefgh") return send(200, { id: "abcdefgh", slug: "fast-pla", title: "Fast PLA" });
  if (url === "/api/v1/presets/zzzzzzzz") return send(500, { type: "about:blank", title: "Internal Server Error", status: 500 });
  if (url.startsWith("/api/v1/presets/")) return send(404, { type: "about:blank", title: "Not Found", status: 404 });
  if (url.includes("nozzle=0.7")) {
    return send(400, { type: "about:blank", title: "Bad Request", status: 400, detail: "nozzle must be one of 0.2 … 1.2" });
  }
  if (url.startsWith("/api/v1/presets?")) return send(200, { items: [], next_cursor: "next" });
  send(500, { type: "about:blank", title: "Internal Server Error", status: 500 });
});
await new Promise<void>((resolve) => api.listen(0, "127.0.0.1", resolve));
const apiURL = `http://127.0.0.1:${(api.address() as AddressInfo).port}`;

beforeEach(() => vi.stubEnv("API_URL", apiURL));
afterEach(() => vi.unstubAllEnvs());
afterAll(() => api.close());

test("presetPath and parsePresetParam agree on /presets/<slug>-<id>", () => {
  expect(presetPath({ slug: "fast-pla-on-a-p1s", id: "ab3defgh" })).toBe("/presets/fast-pla-on-a-p1s-ab3defgh");
  expect(parsePresetParam("fast-pla-on-a-p1s-ab3defgh")).toEqual({ slug: "fast-pla-on-a-p1s", id: "ab3defgh" });
  // A bare ID is a link with the slug cut off. The page then redirects to the full path.
  expect(parsePresetParam("ab3defgh")).toEqual({ slug: "", id: "ab3defgh" });
});

test("parsePresetParam refuses an ID with letters that public IDs never use", () => {
  for (const bad of ["fast-pla", "fast-abcdefg", "fast-abcdefgl", "fast-ABCDEFGH", "fast-abcd0fgh", "fast-abcd1fgh", "-"]) {
    expect(parsePresetParam(bad), bad).toBeNull();
  }
});

test("cleanBrowseQuery keeps the known filters with a value and says when it dropped something", () => {
  expect(cleanBrowseQuery({ printer: "bambu-lab-p1s", nozzle: "0.4", cursor: "c1" })).toEqual({
    query: { printer: "bambu-lab-p1s", nozzle: "0.4" },
    cursor: "c1",
    clean: true,
  });
  // An empty select sends "printer=". The page redirects to the path without it, so shared links stay short.
  expect(cleanBrowseQuery({ printer: "", material: "x", utm_source: "discord", slicer: ["a", "b"] })).toEqual({
    query: { material: "x", slicer: "a" },
    cursor: undefined,
    clean: false,
  });
});

test("browsePath writes the filters in a fixed order and leaves out the empty ones", () => {
  expect(browsePath({ slicer: "orcaslicer", printer: "bambu-lab-p1s" })).toBe("/presets?printer=bambu-lab-p1s&slicer=orcaslicer");
  expect(browsePath({})).toBe("/presets");
  expect(browsePath({ printer: "a b" }, { cursor: "x=" })).toBe("/presets?printer=a+b&cursor=x%3D");
});

test("nozzleLabel writes a nozzle the way a slicer shows it, with the unit kept on the same line", () => {
  expect([0.2, 0.25, 0.4, 1, 1.2].map(nozzleLabel)).toEqual(["0.2\u00a0mm", "0.25\u00a0mm", "0.4\u00a0mm", "1.0\u00a0mm", "1.2\u00a0mm"]);
});

test("authorName gives the display name with the handle, or the handle alone when there is no name", () => {
  expect(authorName({ handle: "mia", display_name: "Mia Hartmann" })).toBe("Mia Hartmann (@mia)");
  expect(authorName({ handle: "mia", display_name: "" })).toBe("@mia");
});

test("getPreset gives the preset, null for a missing one, and fails on an API error", async () => {
  expect(await getPreset("abcdefgh")).toMatchObject({ title: "Fast PLA" });
  expect(await getPreset("bcdefghj")).toBeNull();
  await expect(getPreset("zzzzzzzz")).rejects.toThrow("status 500");
});

test("listPresets sends only the filters and the cursor, and turns a 400 into a message", async () => {
  const page = await listPresets({ printer: "bambu-lab-p1s", cursor: "c1" });
  expect(page).toEqual({ ok: true, page: { items: [], next_cursor: "next" } });
  expect(requests.at(-1)).toBe("/api/v1/presets?printer=bambu-lab-p1s&cursor=c1&limit=20");

  expect(await listPresets({ nozzle: "0.7" })).toEqual({ ok: false, problem: "nozzle must be one of 0.2 … 1.2" });
});

test("isHandle takes only handles that the API can give, so the profile page asks only for those", () => {
  expect(isHandle("mia-prints")).toBe(true);
  expect(isHandle("ab")).toBe(false);
  expect(isHandle("Mia")).toBe(false);
  expect(isHandle("-mia")).toBe(false);
  expect(isHandle("mia--x")).toBe(false);
  expect(isHandle("a".repeat(31))).toBe(false);
  expect(profilePath("mia-prints")).toBe("/u/mia-prints");
});

test("listPresets asks for one author's presets on a profile page", async () => {
  await listPresets({ author: "mia", cursor: "c2" });
  expect(requests.at(-1)).toBe("/api/v1/presets?cursor=c2&author=mia&limit=20");
});
