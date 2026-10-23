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
  expect(screen.getByText("No reviews yet")).toBeDefined();
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
  expect(await screen.findByText("You cannot review your own preset.")).toBeDefined();
  expect(screen.queryByRole("radiogroup")).toBeNull();
});

test("a member rates with the star radios, posts, then updates or removes the review", async () => {
  const writes = stubAPI(member, null, [review("someone", 5, "Great")]);
  render(
    <SessionProvider>
      <Reviews presetId="ab3defgh" authorHandle="maker" initial={[review("someone", 5, "Great")]} />
    </SessionProvider>,
  );
  const group = await screen.findByRole("radiogroup", { name: "Your rating" });
  // Posting without stars says what is missing.
  fireEvent.click(screen.getByRole("button", { name: "Post review" }));
  expect(await screen.findByText("Choose 1 to 5 stars.")).toBeDefined();

  fireEvent.click(within(group).getByLabelText("4 stars"));
  fireEvent.change(screen.getByLabelText("Your review (optional)"), { target: { value: "Strings a little." } });
  fireEvent.click(screen.getByRole("button", { name: "Post review" }));

  await waitFor(() => expect(writes).toEqual([{ method: "POST", body: { stars: 4, body: "Strings a little." } }]));
  await waitFor(() => expect(within(screen.getByRole("list")).getByText("Strings a little.")).toBeDefined());
  expect(screen.getByRole("button", { name: "Update review" })).toBeDefined();

  fireEvent.click(screen.getByRole("button", { name: "Remove my review" }));
  await waitFor(() => expect(writes.at(-1)?.method).toBe("DELETE"));
  await waitFor(() => expect(within(screen.getByRole("list")).queryByText("Strings a little.")).toBeNull());
  expect(screen.getByRole("button", { name: "Post review" })).toBeDefined();
});
