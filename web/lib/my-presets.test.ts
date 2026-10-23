import { afterEach, expect, test, vi } from "vitest";
import { ApiError } from "./account";
import { createPreset, deletePreset, fieldOfProblem, updatePreset, uploadPresetFile } from "./my-presets";

afterEach(() => vi.unstubAllGlobals());

// stubFetch answers every request with one status and body, and keeps the requests.
function stubFetch(status: number, body: unknown) {
  const calls: { url: string; init: RequestInit }[] = [];
  vi.stubGlobal("fetch", async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return new Response(status === 204 ? null : JSON.stringify(body), { status });
  });
  return calls;
}

test("fieldOfProblem finds the form field that an API message is about", () => {
  expect(fieldOfProblem("title must have 5 to 100 characters")).toBe("title");
  expect(fieldOfProblem("printer is not in the catalog")).toBe("printer");
  expect(fieldOfProblem("material is required")).toBe("material");
  expect(fieldOfProblem("nozzle_mm must be one of 0.2, 0.25")).toBe("nozzle_mm");
  expect(fieldOfProblem("slicer_version must have at most 20 characters")).toBe("slicer_version");
  // "slicer_version" starts with "slicer", so the longer name must win.
  expect(fieldOfProblem("slicer is not in the catalog")).toBe("slicer");
  expect(fieldOfProblem("file is not one of your checked uploads")).toBe("file");
  expect(fieldOfProblem("choose a handle first")).toBeNull();
  expect(fieldOfProblem("")).toBeNull();
});

test("the writes send the session cookie, the right method and the JSON body", async () => {
  const calls = stubFetch(201, { id: "ab3defgh", slug: "fast-pla" });
  const input = { title: "Fast PLA", printer: "p", material: "m", nozzle_mm: 0.4 as const, slicer: "s", status: "draft" as const };
  expect(await createPreset(input)).toMatchObject({ id: "ab3defgh" });
  expect(calls[0].url).toMatch(/\/api\/v1\/presets$/);
  expect(calls[0].init).toMatchObject({ method: "POST", credentials: "include" });
  expect(JSON.parse(String(calls[0].init.body))).toEqual(input);

  await updatePreset("ab3defgh", { status: "published" });
  expect(calls[1].url).toMatch(/\/api\/v1\/presets\/ab3defgh$/);
  expect(calls[1].init.method).toBe("PATCH");
});

test("a refused write keeps the API's message for the form", async () => {
  stubFetch(400, { detail: "title must have 5 to 100 characters" });
  await expect(createPreset({ title: "x", printer: "p", material: "m", nozzle_mm: 0.4, slicer: "s" })).rejects.toEqual(
    new ApiError(400, "title must have 5 to 100 characters", null),
  );
});

test("deletePreset refuses an ID that is not a public ID, before it calls the API", async () => {
  const calls = stubFetch(204, null);
  await expect(deletePreset("../me")).rejects.toThrow();
  expect(calls).toHaveLength(0);
  await deletePreset("ab3defgh");
  expect(calls[0].init.method).toBe("DELETE");
});

test("uploadPresetFile asks for a ticket, sends the file to its URL, then completes the upload", async () => {
  const calls: { url: string; init: RequestInit }[] = [];
  vi.stubGlobal("fetch", async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    if (url.endsWith("/api/v1/uploads")) {
      return new Response(JSON.stringify({ id: "abcdefghjkmnpqrs", url: "https://bucket.test/u/x?sig=1", method: "PUT", headers: { "Content-Type": "application/octet-stream" } }), { status: 201 });
    }
    if (url.startsWith("https://bucket.test/")) return new Response(null, { status: 200 });
    return new Response(JSON.stringify({ id: "abcdefghjkmnpqrs", status: "ready", slicer: "prusaslicer" }), { status: 200 });
  });
  const file = new File(["layer_height = 0.2\n"], "my.ini");

  expect(await uploadPresetFile(file)).toMatchObject({ status: "ready", slicer: "prusaslicer" });
  expect(JSON.parse(String(calls[0].init.body))).toEqual({ kind: "preset_file", size: file.size });
  // The file goes to the bucket without the session cookie, with only the signed headers.
  expect(calls[1]).toMatchObject({ url: "https://bucket.test/u/x?sig=1", init: { method: "PUT", body: file, headers: { "Content-Type": "application/octet-stream" } } });
  expect(calls[1].init.credentials).toBeUndefined();
  expect(calls[2].url).toMatch(/\/api\/v1\/uploads\/abcdefghjkmnpqrs\/complete$/);
});

test("uploadPresetFile says plainly when the bucket refuses the file", async () => {
  vi.stubGlobal("fetch", async (url: string) =>
    url.endsWith("/api/v1/uploads")
      ? new Response(JSON.stringify({ id: "abcdefghjkmnpqrs", url: "https://bucket.test/u/x", method: "PUT", headers: {} }), { status: 201 })
      : new Response("denied", { status: 403 }),
  );
  await expect(uploadPresetFile(new File(["x"], "x.ini"))).rejects.toMatchObject({ detail: "The file did not upload. Try again." });
});
