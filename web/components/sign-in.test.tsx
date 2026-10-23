import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";

const params = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useSearchParams: () => params,
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));
vi.mock("./session", () => ({ useSession: () => ({ state: { status: "signed-out" }, refresh: async () => {} }) }));

const { SignIn } = await import("./sign-in");

afterEach(() => {
  for (const k of [...params.keys()]) params.delete(k);
  vi.unstubAllGlobals();
});

test("the provider buttons go to the API with onboarding as the return path", () => {
  params.set("return", "/presets");
  render(<SignIn />);
  const github = new URL(screen.getByRole("link", { name: "Continue with GitHub" }).getAttribute("href")!);
  expect(github.pathname).toBe("/api/v1/auth/github/start");
  expect(github.searchParams.get("return")).toBe("/welcome?return=%2Fpresets");
});

test("an error from the OAuth callback shows in words", () => {
  params.set("error", "provider");
  render(<SignIn />);
  expect(screen.getByRole("alert").textContent).toMatch(/did not answer/);
});

test("sending the email shows the inbox step, and a refused address says how to fix it", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ status: "sent" }), { status: 202 })));
  render(<SignIn />);
  fireEvent.change(screen.getByLabelText("Email address"), { target: { value: "alice@example.com" } });
  fireEvent.click(screen.getByRole("button", { name: "Email me a sign-in link" }));
  expect(await screen.findByText("Check your inbox")).toBeTruthy();

  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ detail: "x" }), { status: 429, headers: { "Retry-After": "600" } })));
  fireEvent.click(screen.getByRole("button", { name: "Use a different address" }));
  fireEvent.change(screen.getByLabelText("Email address"), { target: { value: "alice@example.com" } });
  fireEvent.click(screen.getByRole("button", { name: "Email me a sign-in link" }));
  expect((await screen.findByText(/Too many attempts/)).textContent).toBe("Too many attempts. Try again in 10 minutes.");
});

test("an empty email field says what to do instead of sending", () => {
  const fetchSpy = vi.fn();
  vi.stubGlobal("fetch", fetchSpy);
  render(<SignIn />);
  fireEvent.click(screen.getByRole("button", { name: "Email me a sign-in link" }));
  expect(screen.getByText("Enter your email address.")).toBeTruthy();
  expect(fetchSpy).not.toHaveBeenCalled();
});
