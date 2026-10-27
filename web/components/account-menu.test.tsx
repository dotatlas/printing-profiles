import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";

vi.mock("next/navigation", () => ({ usePathname: () => "/presets", useRouter: () => ({ push: vi.fn() }) }));
vi.mock("./session", () => ({
  useSession: () => ({
    state: { status: "signed-in", me: { handle: "alice-prints", display_name: "Alice", needs_onboarding: false } },
    refresh: async () => {},
  }),
}));

const { AccountMenu } = await import("./account-menu");

test("the handle opens the menu, and Escape closes it and returns the focus", () => {
  render(<AccountMenu />);
  const button = screen.getByRole("button", { name: /@alice-prints/ });
  expect(button.getAttribute("aria-expanded")).toBe("false");

  fireEvent.click(button);
  expect(button.getAttribute("aria-expanded")).toBe("true");
  expect(screen.getByRole("link", { name: "Settings" })).toBeTruthy();
  expect(screen.getByRole("link", { name: "My bookmarks" }).getAttribute("href")).toBe("/my-bookmarks");

  fireEvent.keyDown(document, { key: "Escape" });
  expect(button.getAttribute("aria-expanded")).toBe("false");
  expect(document.activeElement).toBe(button);
});

test("the menu closes when the focus moves out of it", () => {
  render(
    <>
      <AccountMenu />
      <button type="button">Elsewhere</button>
    </>,
  );
  const button = screen.getByRole("button", { name: /@alice-prints/ });
  fireEvent.click(button);
  expect(button.getAttribute("aria-expanded")).toBe("true");
  fireEvent.focusIn(screen.getByRole("button", { name: "Elsewhere" }));
  expect(button.getAttribute("aria-expanded")).toBe("false");
});
