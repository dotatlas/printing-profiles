// @vitest-environment node
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { afterAll, afterEach, beforeEach, expect, test, vi } from "vitest";
import { listFirst } from "./api";

// The fake API answers the slicers route and fails every other route, like an API with a broken database.
const requests: string[] = [];
const api = createServer((req, res) => {
  requests.push(req.url ?? "");
  if (req.url?.startsWith("/api/v1/slicers?")) {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ items: [{ slug: "orcaslicer", name: "OrcaSlicer" }] }));
    return;
  }
  res.writeHead(500, { "Content-Type": "application/problem+json" });
  res.end(JSON.stringify({ type: "about:blank", title: "Internal Server Error", status: 500 }));
});
await new Promise<void>((resolve) => api.listen(0, "127.0.0.1", resolve));
const apiURL = `http://127.0.0.1:${(api.address() as AddressInfo).port}`;

beforeEach(() => vi.stubEnv("API_URL", apiURL));
afterEach(() => vi.unstubAllEnvs());
afterAll(() => api.close());

test("listFirst asks for one page of 100 and gives back its items", async () => {
  const items = await listFirst("/api/v1/slicers");

  expect(items).toEqual([{ slug: "orcaslicer", name: "OrcaSlicer" }]);
  expect(requests.at(-1)).toBe("/api/v1/slicers?limit=100");
});

test("listFirst fails with the route and the status when the API fails", async () => {
  await expect(listFirst("/api/v1/printers")).rejects.toThrow("GET /api/v1/printers: status 500");
});

test("in production, listFirst needs API_URL and never falls back to localhost", async () => {
  vi.stubEnv("API_URL", "");
  vi.stubEnv("NODE_ENV", "production");

  await expect(listFirst("/api/v1/slicers")).rejects.toThrow("API_URL must be set in production");
});
