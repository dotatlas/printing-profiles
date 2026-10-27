import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import type { Comment, Me } from "../lib/api-types";
import { Comments } from "./comments";
import { SessionProvider } from "./session";

vi.mock("next/navigation", () => ({
  usePathname: () => "/presets/fast-pla-ab3defgh",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), refresh: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => vi.unstubAllGlobals());

const at = "2026-10-20T10:00:00Z";
const c = (id: string, handle: string, body: string, replies: Comment[] = []): Comment => ({
  id,
  body,
  author: { handle, display_name: "" },
  created_at: at,
  edited: false,
  deleted: false,
  replies,
});

// stubAPI answers /me with this member (or 401) and keeps a comment list that the writes change.
function stubAPI(me: Me | null, start: Comment[]) {
  let items = start;
  const writes: { method: string; url: string; body: unknown }[] = [];
  vi.stubGlobal("fetch", async (url: string, init?: RequestInit) => {
    const json = (status: number, body: unknown) => new Response(status === 204 ? null : JSON.stringify(body), { status });
    if (url.endsWith("/api/v1/me")) return me ? json(200, me) : json(401, {});
    const method = init?.method ?? "GET";
    if (method === "GET") return json(200, { items });
    const body = init?.body ? JSON.parse(String(init.body)) : null;
    writes.push({ method, url, body });
    if (method === "POST") {
      const made = c(`zzzzzzzzzzz${writes.length + 1}`, me?.handle ?? "", body.body); // IDs use a-z and 2-9 without look-alikes
      items = body.parent
        ? items.map((t) => (t.id === body.parent ? { ...t, replies: [...(t.replies ?? []), made] } : t))
        : [...items, made];
      return json(201, made);
    }
    if (method === "PATCH") {
      items = items.map((t) => (url.endsWith(t.id) ? { ...t, body: body.body, edited: true } : t));
      return json(200, items[0]);
    }
    items = items.map((t) => (url.endsWith(t.id) ? { ...t, deleted: true, body: "", author: { handle: "", display_name: "" } } : t));
    return json(204, null);
  });
  return writes;
}

const member: Me = { handle: "tester", display_name: "", needs_onboarding: false };

test("a visitor reads the thread and gets a sign-in link", async () => {
  stubAPI(null, []);
  render(
    <SessionProvider>
      <Comments presetId="ab3defgh" initial={[c("aaaaaaaaaaaa", "asker", "Which plate?", [c("bbbbbbbbbbbb", "maker", "Textured PEI.")])]} />
    </SessionProvider>,
  );
  expect(screen.getByText("Which plate?")).toBeDefined();
  expect(screen.getByText("Textured PEI.")).toBeDefined();
  expect(screen.getByRole("link", { name: "@maker" }).getAttribute("href")).toBe("/u/maker");
  expect(await screen.findByRole("link", { name: "Sign in to comment" })).toBeDefined();
  expect(screen.queryByRole("button", { name: /^Reply/ })).toBeNull();
});

test("a member comments, replies, edits and deletes their own comment", async () => {
  const writes = stubAPI(member, [c("aaaaaaaaaaaa", "asker", "Which plate?")]);
  render(
    <SessionProvider>
      <Comments presetId="ab3defgh" initial={[c("aaaaaaaaaaaa", "asker", "Which plate?")]} />
    </SessionProvider>,
  );
  // Another member's comment has Reply but no Edit or Delete.
  const first = (await screen.findByText("Which plate?")).closest("article") as HTMLElement;
  expect(within(first).queryByRole("button", { name: /^Edit/ })).toBeNull();

  fireEvent.click(within(first).getByRole("button", { name: /^Reply/ }));
  fireEvent.change(within(first).getByLabelText("Your reply"), { target: { value: "Smooth PEI works too." } });
  fireEvent.click(within(first).getByRole("button", { name: "Post reply" }));
  await waitFor(() => expect(screen.getByText("Smooth PEI works too.")).toBeDefined());
  expect(writes[0].body).toEqual({ body: "Smooth PEI works too.", parent: "aaaaaaaaaaaa" });

  fireEvent.change(screen.getByLabelText("Add a comment"), { target: { value: "Great preset." } });
  fireEvent.click(screen.getByRole("button", { name: "Post comment" }));
  // The selector skips the text box, which still holds the text while the post runs.
  const mine = (await screen.findByText("Great preset.", { selector: "p" })).closest("article") as HTMLElement;

  fireEvent.click(within(mine).getByRole("button", { name: /^Edit/ }));
  fireEvent.change(within(mine).getByLabelText("Edit your comment"), { target: { value: "Great preset, thanks." } });
  fireEvent.click(within(mine).getByRole("button", { name: "Save" }));
  await waitFor(() => expect(writes.at(-1)?.method).toBe("PATCH"));

  const again = (await screen.findByText("Great preset, thanks.", { selector: "p" })).closest("article") as HTMLElement;
  fireEvent.click(within(again).getByRole("button", { name: /^Delete/ }));
  // The delete asks first, with the focus on Cancel.
  expect(document.activeElement?.textContent).toBe("Cancel");
  fireEvent.click(within(again).getByRole("button", { name: "Delete for good" }));
  await waitFor(() => expect(writes.at(-1)?.method).toBe("DELETE"));
  expect(await screen.findByText("Comment deleted")).toBeDefined();
  // A deleted comment without replies leaves the thread, so the status says it and the focus goes to the heading.
  expect(screen.getByRole("status").textContent).toBe("Your comment is deleted.");
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Comments" }));
});

test("an empty comment is refused before any call", async () => {
  const writes = stubAPI(member, []);
  render(
    <SessionProvider>
      <Comments presetId="ab3defgh" initial={[]} />
    </SessionProvider>,
  );
  fireEvent.click(await screen.findByRole("button", { name: "Post comment" }));
  expect(screen.getByText("Write something first.")).toBeDefined();
  expect(writes).toHaveLength(0);
});
