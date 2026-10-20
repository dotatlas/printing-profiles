import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // Components render in jsdom. A test file can ask for the node environment in its first comment.
    environment: "jsdom",
    // Testing Library removes the rendered components after each test only when the test globals exist.
    globals: true,
  },
});
