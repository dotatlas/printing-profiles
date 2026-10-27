import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import type { Me } from "../lib/api-types";
import { BookmarkButton } from "./bookmark-button";
import { SessionProvider } from "./session";

vi.mock("next/navigation", () => ({
  usePathname: () => "/presets/fast-pla-ab3defgh",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => vi.unstubAllGlobals());

// stubAPI answers /me with this member (or 401), the bookmark check with saved, and each write with fail or 204.
function stubAPI(me: Me | null, saved: boolean, fail = false) {
  const writes: string[] = [];
  vi.stubGlobal("fetch", async (url: string, init?: RequestInit) => {
    const json = (status: number, body: unknown) => new Response(status === 204 ? null : JSON.stringify(body), { status });
    if (url.endsWith("/api/v1/me")) return me ? json(200, me) : json(401, {});
    const method = init?.method ?? "GET";
    if (method === "GET") return json(200, { bookmarked: saved });
    writes.push(method);
    return fail ? json(500, {}) : json(204, null);
  });
  return writes;
}

const member: Me = { handle: "tester", display_name: "", needs_onboarding: false };

test("a visitor gets a sign-in link that comes back to the preset", async () => {
  stubAPI(null, false);
  render(
    <SessionProvider>
      <BookmarkButton presetId="ab3defgh" />
    </SessionProvider>,
  );
  const link = await screen.findByRole("link", { name: "Bookmark" });
  expect(link.getAttribute("href")).toBe(`/signin?return=${encodeURIComponent("/presets/fast-pla-ab3defgh")}`);
});

test("a member bookmarks a preset and removes the bookmark", async () => {
  const writes = stubAPI(member, false);
  render(
    <SessionProvider>
      <BookmarkButton presetId="ab3defgh" />
    </SessionProvider>,
  );
  const button = await screen.findByRole("button", { name: "Bookmark" });
  expect(button.getAttribute("aria-pressed")).toBe("false");
  fireEvent.click(button);
  await waitFor(() => expect(button.getAttribute("aria-pressed")).toBe("true"));
  expect(writes).toEqual(["POST"]);
  fireEvent.click(button);
  await waitFor(() => expect(button.getAttribute("aria-pressed")).toBe("false"));
  expect(writes).toEqual(["POST", "DELETE"]);
});

test("a failed bookmark goes back and says so", async () => {
  stubAPI(member, true, true);
  render(
    <SessionProvider>
      <BookmarkButton presetId="ab3defgh" />
    </SessionProvider>,
  );
  const button = await screen.findByRole("button", { name: "Bookmark" });
  await waitFor(() => expect(button.getAttribute("aria-pressed")).toBe("true"));
  fireEvent.click(button);
  expect(await screen.findByText("The bookmark did not change. Try again.")).toBeDefined();
  expect(button.getAttribute("aria-pressed")).toBe("true");
});
