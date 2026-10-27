import { afterEach, expect, test, vi } from "vitest";
import { addBookmark, deleteMyReview, getMyReview, isBookmarked, listReviews, removeBookmark, saveReview, starsLabel } from "./community";

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

test("starsLabel says a rating in words, for screen readers", () => {
  expect(starsLabel(1)).toBe("1 star out of 5");
  expect(starsLabel(4)).toBe("4 stars out of 5");
  expect(starsLabel(4.5)).toBe("4.5 stars out of 5");
});

test("saveReview posts the stars and the text with the session", async () => {
  const calls = stubFetch(201, { stars: 4, body: "Good", author: { handle: "a", display_name: "" }, created_at: "", edited: false });
  await saveReview("ab3defgh", 4, "Good");
  expect(calls[0].url).toMatch(/\/api\/v1\/presets\/ab3defgh\/reviews$/);
  expect(calls[0].init).toMatchObject({ method: "POST", credentials: "include" });
  expect(JSON.parse(String(calls[0].init.body))).toEqual({ stars: 4, body: "Good" });
});

test("getMyReview gives null when the member has not reviewed the preset", async () => {
  stubFetch(404, { detail: "you have not reviewed this preset" });
  expect(await getMyReview("ab3defgh")).toBeNull();
});

test("the review calls refuse an ID that is not a public ID, before they call the API", async () => {
  const calls = stubFetch(204, null);
  await expect(deleteMyReview("../me")).rejects.toThrow();
  await expect(listReviews("../me")).rejects.toThrow();
  expect(calls).toHaveLength(0);
});

test("the bookmark calls add, check and remove one preset with the session", async () => {
  const calls = stubFetch(200, { bookmarked: true });
  expect(await isBookmarked("ab3defgh")).toBe(true);
  expect(calls[0].url).toMatch(/\/api\/v1\/me\/bookmarks\/ab3defgh$/);
  await addBookmark("ab3defgh");
  expect(calls[1].init).toMatchObject({ method: "POST", credentials: "include" });
  expect(JSON.parse(String(calls[1].init.body))).toEqual({ preset: "ab3defgh" });
  await removeBookmark("ab3defgh");
  expect(calls[2].url).toMatch(/\/api\/v1\/me\/bookmarks\/ab3defgh$/);
  expect(calls[2].init).toMatchObject({ method: "DELETE" });
  await expect(removeBookmark("../me")).rejects.toThrow();
  expect(calls).toHaveLength(3);
});
