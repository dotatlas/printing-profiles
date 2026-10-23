import { render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { SessionProvider } from "./session";
import { handleProblem, toHandle, Welcome } from "./welcome";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => vi.unstubAllGlobals());

test("toHandle shapes typed text into a handle as the visitor types", () => {
  expect(toHandle("Alice Prints")).toBe("alice-prints");
  expect(toHandle("my_PLA__tuner")).toBe("my-pla-tuner");
  expect(toHandle("émile!!")).toBe("emile");
  expect(toHandle("Aleksandra Wiśniewska")).toBe("aleksandra-wisniewska");
  expect(toHandle("a".repeat(40))).toHaveLength(30);
});

test("handleProblem matches the API's handle rule", () => {
  expect(handleProblem("ab")).not.toBe("");
  expect(handleProblem("-ab")).not.toBe("");
  expect(handleProblem("ab-")).not.toBe("");
  expect(handleProblem("abc")).toBe("");
  expect(handleProblem("pla-tuner-2")).toBe("");
});

test("the terms link says that it opens a new tab, with a space before the hidden part", async () => {
  vi.stubGlobal("fetch", async () => new Response(JSON.stringify({ handle: "", display_name: "", needs_onboarding: true })));
  render(
    <SessionProvider>
      <Welcome />
    </SessionProvider>,
  );

  expect(await screen.findByRole("link", { name: "terms of use (opens in a new tab)" })).toBeDefined();
});
