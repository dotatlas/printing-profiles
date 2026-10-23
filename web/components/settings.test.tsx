import { render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { SessionProvider } from "./session";
import { Settings } from "./settings";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => vi.unstubAllGlobals());

test("the Remove and Sign out buttons name what they act on, with a space before the hidden part", async () => {
  const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200 });
  vi.stubGlobal("fetch", async (url: string) => {
    if (url.endsWith("/api/v1/me")) return json({ handle: "maker", display_name: "Mia", needs_onboarding: false });
    if (url.endsWith("/api/v1/me/identities")) {
      return json({
        items: [
          { id: 1, provider: "github", created_at: "2026-10-01T10:00:00Z" },
          { id: 2, provider: "discord", created_at: "2026-10-02T10:00:00Z" },
        ],
      });
    }
    return json({
      items: [
        { id: 1, user_agent: "", created_at: "2026-10-01T10:00:00Z", last_seen_at: new Date().toISOString(), current: true },
        {
          id: 2,
          user_agent: "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0",
          created_at: "2026-10-01T10:00:00Z",
          last_seen_at: new Date().toISOString(),
          current: false,
        },
      ],
    });
  });
  render(
    <SessionProvider>
      <Settings />
    </SessionProvider>,
  );

  expect(await screen.findByRole("button", { name: "Remove GitHub" })).toBeDefined();
  expect(await screen.findByRole("button", { name: "Sign out Firefox on Linux, active just now" })).toBeDefined();
});
