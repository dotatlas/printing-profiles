import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import type { Me, Review } from "../lib/api-types";
import { Reviews } from "./reviews";
import { SessionProvider } from "./session";
import { RatingSummary } from "./stars";

vi.mock("next/navigation", () => ({
  usePathname: () => "/presets/fast-pla-ab3defgh",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => vi.unstubAllGlobals());

const review = (handle: string, stars: number, body: string): Review => ({
  stars,
  body,
  author: { handle, display_name: "" },
  created_at: "2026-10-20T10:00:00Z",
  edited: false,
});

// stubAPI answers /me with this member (or 401), the member's review, and the reviews list; it keeps the writes.
function stubAPI(me: Me | null, mine: Review | null, list: Review[]) {
  const writes: { method: string; body: unknown }[] = [];
  let items = list;
  vi.stubGlobal("fetch", async (url: string, init?: RequestInit) => {
    const json = (status: number, body: unknown) => new Response(status === 204 ? null : JSON.stringify(body), { status });
    if (url.endsWith("/api/v1/me")) return me ? json(200, me) : json(401, {});
    if (url.endsWith("/reviews/mine") && (!init?.method || init.method === "GET")) return mine ? json(200, mine) : json(404, {});
    if (init?.method === "POST") {
      const body = JSON.parse(String(init.body));
      writes.push({ method: "POST", body });
      const saved = review(me?.handle ?? "", body.stars, body.body);
      items = [saved, ...items.filter((r) => r.author.handle !== saved.author.handle)];
      return json(201, saved);
    }
    if (init?.method === "DELETE") {
      writes.push({ method: "DELETE", body: null });
      items = items.filter((r) => r.author.handle !== me?.handle);
      return json(204, null);
    }
    return json(200, { items });
  });
  return writes;
}

const member: Me = { handle: "tester", display_name: "Tess", needs_onboarding: false };

test("the rating summary says the average and the count, and links to the reviews", () => {
  render(<RatingSummary rating={{ count: 12, average: 4.5 }} />);
  expect(screen.getByRole("link", { name: "Rated 4.5 stars out of 5 from 12 reviews" })).toHaveProperty("href", expect.stringMatching(/#reviews$/));
  render(<RatingSummary />);
  expect(screen.getByRole("link", { name: "No reviews yet" })).toHaveProperty("href", expect.stringMatching(/#reviews$/));
});

test("the reviews list shows each review's stars in words and its text", () => {
  stubAPI(null, null, []);
  render(
    <SessionProvider>
      <Reviews presetId="ab3defgh" authorHandle="maker" initial={[review("someone", 4, "Clean first layer.\nNo warping.")]} />
    </SessionProvider>,
  );
  const item = screen.getByRole("listitem");
  expect(within(item).getByText("4 stars out of 5")).toBeDefined();
  expect(within(item).getByText(/Clean first layer\./)).toBeDefined();
  expect(within(item).getByRole("link", { name: "@someone" }).getAttribute("href")).toBe("/u/someone");
});

test("a signed-out visitor gets a sign-in link, and the author gets no form", async () => {
  stubAPI(null, null, []);
  const { unmount } = render(
    <SessionProvider>
      <Reviews presetId="ab3defgh" authorHandle="maker" initial={[]} />
    </SessionProvider>,
  );
  expect(await screen.findByRole("link", { name: "Sign in to review" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/signin\?return=%2Fpresets%2Ffast-pla-ab3defgh%23reviews$/),
  );
  unmount();

  stubAPI({ ...member, handle: "maker" }, null, []);
  render(
    <SessionProvider>
      <Reviews presetId="ab3defgh" authorHandle="maker" initial={[]} />
    </SessionProvider>,
  );
  expect(await screen.findByText("Reviews come from people who printed your preset.")).toBeDefined();
  expect(screen.queryByRole("radiogroup")).toBeNull();
});

test("a member opens the form, posts, sees the review once, edits it and removes it after a confirm", async () => {
  const writes = stubAPI(member, null, [review("someone", 5, "Great")]);
  render(
    <SessionProvider>
      <Reviews presetId="ab3defgh" authorHandle="maker" initial={[review("someone", 5, "Great")]} />
    </SessionProvider>,
  );
  // The list comes first; the form opens on request.
  expect(screen.queryByRole("radiogroup")).toBeNull();
  fireEvent.click(await screen.findByRole("button", { name: "Write a review" }));
  const group = screen.getByRole("radiogroup", { name: "Your rating" });
  // Posting without stars says what is missing.
  fireEvent.click(screen.getByRole("button", { name: "Post review" }));
  expect(await screen.findByText("Choose 1 to 5 stars.")).toBeDefined();

  fireEvent.click(within(group).getByLabelText("4 stars"));
  fireEvent.change(screen.getByLabelText("Your review (optional)"), { target: { value: "Strings a little." } });
  fireEvent.click(screen.getByRole("button", { name: "Post review" }));
  await waitFor(() => expect(writes).toEqual([{ method: "POST", body: { stars: 4, body: "Strings a little." } }]));

  // The member's review shows once, in its own panel, and the list holds only the other reviews.
  const mine = await screen.findByRole("region", { name: "Your review" });
  expect(within(mine).getByText("Strings a little.")).toBeDefined();
  expect(document.activeElement?.textContent).toBe("Your review");
  await waitFor(() => expect(within(screen.getByRole("list")).getAllByRole("listitem")).toHaveLength(1));
  expect(screen.getByRole("status").textContent).toBe("Your review is posted.");

  fireEvent.click(within(mine).getByRole("button", { name: "Edit review" }));
  expect((screen.getByLabelText("Your review (optional)") as HTMLTextAreaElement).value).toBe("Strings a little.");
  fireEvent.change(screen.getByLabelText("Your review (optional)"), { target: { value: "Strings a little on PETG." } });
  fireEvent.click(screen.getByRole("button", { name: "Update review" }));
  expect(await screen.findByText("Strings a little on PETG.")).toBeDefined();

  fireEvent.click(screen.getByRole("button", { name: "Remove review" }));
  // The remove asks first, with the focus on Cancel.
  expect(document.activeElement?.textContent).toBe("Cancel");
  fireEvent.click(screen.getByRole("button", { name: "Remove for good" }));
  await waitFor(() => expect(writes.at(-1)?.method).toBe("DELETE"));
  expect(await screen.findByRole("button", { name: "Write a review" })).toBeDefined();
  expect(screen.getByRole("status").textContent).toBe("Your review is removed.");
});

test("more than one page of reviews shows more on request", async () => {
  const all = [review("r1", 5, "First."), review("r2", 4, "Second."), review("r3", 3, "Third.")];
  vi.stubGlobal("fetch", async (url: string) => {
    if (url.endsWith("/api/v1/me")) return new Response("{}", { status: 401 });
    const from = Number(new URL(url).searchParams.get("cursor") ?? 0);
    const page = { items: all.slice(from, from + 2), next_cursor: from + 2 < all.length ? String(from + 2) : undefined };
    return new Response(JSON.stringify(page), { status: 200 });
  });
  render(
    <SessionProvider>
      <Reviews presetId="ab3defgh" authorHandle="maker" initial={all.slice(0, 2)} next="2" />
    </SessionProvider>,
  );
  fireEvent.click(await screen.findByRole("button", { name: "Show more reviews" }));
  expect(await screen.findByText("Third.")).toBeDefined();
  expect(screen.queryByRole("button", { name: "Show more reviews" })).toBeNull();
});
