import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import ErrorPage from "./error";

test("the error page offers a retry and shows the error ID", () => {
  const retry = vi.fn();
  render(<ErrorPage error={Object.assign(new Error("API down"), { digest: "4021967" })} retry={retry} />);

  fireEvent.click(screen.getByRole("button", { name: "Try again" }));

  expect(retry).toHaveBeenCalledOnce();
  expect(screen.getByText("Error ID: 4021967")).toBeTruthy();
  expect(screen.queryByText("API down"), "the error message can hold internal details").toBeNull();
});
