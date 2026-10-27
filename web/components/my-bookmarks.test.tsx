import { render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import type { Me, Preset } from "../lib/api-types";
import { MyBookmarks } from "./my-bookmarks";
import { SessionProvider } from "./session";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => "/my-bookmarks",
  useRouter: () => ({ push: vi.fn(), replace, refresh: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => vi.unstubAllGlobals());

const bambu = { slug: "bambu-lab", name: "Bambu Lab" };
const preset: Preset = {
  id: "ab3defgh",
  slug: "fast-pla",
  title: "Fast PLA for the P1S",
  description: "",
  author: { handle: "maker", display_name: "Mia Maker" },
  printer: { slug: "bambu-lab-p1s", model: "P1S", brand: bambu },
  material: { slug: "bambu-lab-pla-basic", product_name: "PLA Basic", type: "PLA", brand: bambu },
  nozzle_mm: 0.4,
  slicer: { slug: "orcaslicer", name: "OrcaSlicer" },
  slicer_version: "",
  license: "CC-BY-4.0",
  status: "published",
  created_at: "2026-10-20T10:00:00Z",
  updated_at: "2026-10-20T10:00:00Z",
  download_count: 0,
  published_at: "2026-10-20T10:00:00Z",
};

// stubAPI answers /me with this member (or 401) and the bookmark list with these presets.
function stubAPI(me: Me | null, items: Preset[]) {
  vi.stubGlobal("fetch", async (url: string) => {
    if (url.endsWith("/api/v1/me")) return new Response(JSON.stringify(me ?? {}), { status: me ? 200 : 401 });
    return new Response(JSON.stringify({ items }), { status: 200 });
  });
}

const member: Me = { handle: "tester", display_name: "", needs_onboarding: false };

test("a member sees the bookmarked presets as rows", async () => {
  stubAPI(member, [preset]);
  render(
    <SessionProvider>
      <MyBookmarks />
    </SessionProvider>,
  );
  expect((await screen.findByRole("link", { name: "Fast PLA for the P1S" })).getAttribute("href")).toBe("/presets/fast-pla-ab3defgh");
});

test("with no bookmarks, the page says how to make one", async () => {
  stubAPI(member, []);
  render(
    <SessionProvider>
      <MyBookmarks />
    </SessionProvider>,
  );
  expect(await screen.findByText("No bookmarks yet")).toBeDefined();
  expect(screen.getByRole("link", { name: "Browse presets" }).getAttribute("href")).toBe("/presets");
});

test("a visitor goes to sign in, with the way back", async () => {
  stubAPI(null, []);
  render(
    <SessionProvider>
      <MyBookmarks />
    </SessionProvider>,
  );
  await vi.waitFor(() => expect(replace).toHaveBeenCalledWith(`/signin?return=${encodeURIComponent("/my-bookmarks")}`));
});
