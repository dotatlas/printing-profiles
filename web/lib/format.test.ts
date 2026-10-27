import { expect, test } from "vitest";
import { tidyText } from "./format";

test("tidyText keeps one blank line at most and drops blank lines at the ends", () => {
  expect(tidyText("\n\nFirst layer.\n\n\n\n\nWalls.\r\n\r\n\r\nSeam.\n\n")).toBe("First layer.\n\nWalls.\n\nSeam.");
  expect(tidyText("One line.\nNext line.")).toBe("One line.\nNext line.");
});
