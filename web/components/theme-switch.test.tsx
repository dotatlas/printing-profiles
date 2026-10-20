import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";
import { themeScript } from "./theme";
import { ThemeSwitch } from "./theme-switch";

afterEach(() => {
  localStorage.clear();
  delete document.documentElement.dataset.theme;
});

test("a choice shows the theme and saves it, and system removes both", async () => {
  render(<ThemeSwitch />);

  fireEvent.click(screen.getByRole("button", { name: "Dark theme" }));
  expect(document.documentElement.dataset.theme).toBe("dark");
  expect(localStorage.getItem("theme")).toBe("dark");
  await screen.findByRole("button", { name: "Dark theme", pressed: true });

  fireEvent.click(screen.getByRole("button", { name: "System theme" }));
  expect(document.documentElement.dataset.theme).toBeUndefined();
  expect(localStorage.getItem("theme")).toBeNull();
  await screen.findByRole("button", { name: "System theme", pressed: true });
});

test("the pre-paint script applies only a saved light or dark choice", () => {
  for (const [saved, want] of [["dark", "dark"], ["light", "light"], ["purple", undefined]]) {
    localStorage.setItem("theme", saved as string);
    delete document.documentElement.dataset.theme;

    new Function(themeScript)();

    expect(document.documentElement.dataset.theme, `saved ${saved}`).toBe(want);
  }
});
