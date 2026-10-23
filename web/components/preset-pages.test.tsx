import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, test, vi } from "vitest";
import type { Me, Preset } from "../lib/api-types";
import { AuthorActions, EditPreset, MissingOrDraft, MyPresets, SavedNotice } from "./preset-pages";
import { SessionProvider } from "./session";

const replace = vi.fn();
const push = vi.fn();
let search = "";
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace }),
  usePathname: () => "/presets/fast-pla-ab3defgh",
  useSearchParams: () => new URLSearchParams(search),
}));

const bambu = { slug: "bambu-lab", name: "Bambu Lab" };
const preset: Preset = {
  id: "ab3defgh",
  slug: "fast-pla",
  title: "Fast PLA for the P1S",
  author: { handle: "maker", display_name: "Mia Maker" },
  printer: { slug: "bambu-lab-p1s", model: "P1S", brand: bambu },
  material: { slug: "bambu-lab-pla-basic", product_name: "PLA Basic", type: "PLA", brand: bambu },
  nozzle_mm: 0.4,
  slicer: { slug: "orcaslicer", name: "OrcaSlicer" },
  slicer_version: "",
  license: "CC-BY-4.0",
  status: "draft",
  created_at: "2026-10-20T10:00:00Z",
  updated_at: "2026-10-20T10:00:00Z",
};
const catalog = { printers: [], materials: [], slicers: [] };

// signIn makes the fake API answer /me with this user (or 401 for null), the preset route with the draft, and the
// list of the user's presets.
function signIn(me: Me | null) {
  const sent: { method: string; body: unknown }[] = [];
  const published = { ...preset, id: "bcdefghj", slug: "old-pla", title: "Old PLA", status: "published" as const };
  vi.stubGlobal("fetch", async (url: string, init?: RequestInit) => {
    const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });
    if (init?.method && init.method !== "GET") sent.push({ method: init.method, body: JSON.parse(String(init.body)) });
    if (url.endsWith("/api/v1/me")) return me ? json(200, me) : json(401, {});
    if (url.endsWith("/api/v1/me/presets")) return json(200, { items: [preset, published] });
    if (url.endsWith("/api/v1/presets/ab3defgh")) {
      if (init?.method === "PATCH") return json(200, { ...preset, status: "published" });
      return me ? json(200, preset) : json(404, {});
    }
    return json(404, {});
  });
  return sent;
}
const author: Me = { handle: "maker", display_name: "Mia Maker", needs_onboarding: false };
const stranger: Me = { handle: "someone-else", display_name: "", needs_onboarding: false };

beforeEach(() => {
  replace.mockClear();
  push.mockClear();
  search = "";
});
afterEach(() => vi.unstubAllGlobals());

test("the Edit button shows only to the author of the preset", async () => {
  signIn(author);
  const { unmount } = render(
    <SessionProvider>
      <AuthorActions preset={preset} />
    </SessionProvider>,
  );
  expect(await screen.findByRole("link", { name: "Edit preset" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\/fast-pla-ab3defgh\/edit$/),
  );
  unmount();

  signIn(stranger);
  render(
    <SessionProvider>
      <AuthorActions preset={preset} />
      <p>ready</p>
    </SessionProvider>,
  );
  await screen.findByText("ready");
  await new Promise((r) => setTimeout(r, 20));
  expect(screen.queryByRole("link", { name: "Edit preset" })).toBeNull();
});

test("a stranger on the edit page reads that the preset is not theirs, and gets no form", async () => {
  signIn(stranger);
  render(
    <SessionProvider>
      <EditPreset catalog={catalog} id="ab3defgh" />
    </SessionProvider>,
  );
  expect(await screen.findByText(/You can edit only the presets that you shared/)).toBeDefined();
  expect(screen.queryByLabelText("Title")).toBeNull();
});

test("the author sees a draft on its own path, marked as a draft", async () => {
  signIn(author);
  render(
    <SessionProvider>
      <MissingOrDraft />
    </SessionProvider>,
  );
  expect(await screen.findByText("This is a draft. Only you can see it. Publish it when it is ready.")).toBeDefined();
  expect(screen.getByRole("heading", { level: 1, name: "Fast PLA for the P1S" })).toBeDefined();
});

test("a signed-out visitor on a draft path gets the not-found text", async () => {
  signIn(null);
  render(
    <SessionProvider>
      <MissingOrDraft />
    </SessionProvider>,
  );
  expect(await screen.findByRole("heading", { name: "Page not found" })).toBeDefined();
});

test("My presets lists drafts with their mark, and sends a signed-out visitor to sign-in", async () => {
  signIn(author);
  const { unmount } = render(
    <SessionProvider>
      <MyPresets />
    </SessionProvider>,
  );
  expect(await screen.findByRole("heading", { name: "Drafts" })).toBeDefined();
  expect(screen.getByRole("heading", { name: "Published" })).toBeDefined();
  expect(screen.getByText("Draft")).toBeDefined();
  expect(screen.getByRole("link", { name: "Edit Fast PLA for the P1S" })).toBeDefined();
  unmount();

  signIn(null);
  render(
    <SessionProvider>
      <MyPresets />
    </SessionProvider>,
  );
  await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/signin?return=%2Fmy-presets"));
});

test("the draft view publishes the draft in one step", async () => {
  const sent = signIn(author);
  render(
    <SessionProvider>
      <MissingOrDraft />
    </SessionProvider>,
  );

  fireEvent.click(await screen.findByRole("button", { name: "Publish" }));

  // The browser check covers the page load that follows; jsdom cannot navigate.
  await vi.waitFor(() => expect(sent).toEqual([{ method: "PATCH", body: { status: "published" } }]));
});

test("after a publish the page says so, takes the focus, and copies the link", async () => {
  search = "saved=published";
  const writeText = vi.fn(async () => {});
  vi.stubGlobal("navigator", { clipboard: { writeText } });
  render(<SavedNotice />);

  const notice = screen.getByText(/Published\. Anyone can find it now\./);
  expect(document.activeElement).toBe(notice.closest("[tabindex]"));
  fireEvent.click(screen.getByRole("button", { name: "Copy link" }));
  await vi.waitFor(() => expect(screen.getByRole("button", { name: "Link copied" })).toBeDefined());
  expect(writeText).toHaveBeenCalledWith(`${location.origin}${location.pathname}`);
});

test("without a saved mark the notice shows nothing", () => {
  const { container } = render(<SavedNotice />);
  expect(container.innerHTML).toBe("");
});
