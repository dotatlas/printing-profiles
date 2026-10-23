import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import type { Material, Preset, Printer, Slicer } from "../lib/api-types";
import { PresetForm } from "./preset-form";

const bambu = { slug: "bambu-lab", name: "Bambu Lab" };
const printers: Printer[] = [{ slug: "bambu-lab-p1s", model: "P1S", brand: bambu }];
const materials: Material[] = [
  { slug: "bambu-lab-pla-basic", product_name: "PLA Basic", type: "PLA", diameter_mm: 1.75, brand: bambu, colors: [] },
];
const slicers: Slicer[] = [
  { slug: "orcaslicer", name: "OrcaSlicer" },
  { slug: "prusaslicer", name: "PrusaSlicer" },
];
const catalog = { printers, materials, slicers };

const preset: Preset = {
  id: "ab3defgh",
  slug: "fast-pla",
  title: "Fast PLA for the P1S",
  description: "Line one.",
  author: { handle: "maker", display_name: "Mia Maker" },
  printer: printers[0],
  material: { slug: "bambu-lab-pla-basic", product_name: "PLA Basic", type: "PLA", brand: bambu },
  nozzle_mm: 0.6,
  slicer: slicers[0],
  slicer_version: "2.3.0",
  license: "CC0-1.0",
  status: "draft",
  created_at: "2026-10-20T10:00:00Z",
  updated_at: "2026-10-20T10:00:00Z",
  download_count: 0,
};

afterEach(() => {
  vi.unstubAllGlobals();
  sessionStorage.clear();
});

// stubAPI answers every request with one status and body, and keeps the request bodies.
function stubAPI(status: number, body: unknown, headers: Record<string, string> = {}) {
  const sent: unknown[] = [];
  vi.stubGlobal("fetch", async (_url: string, init: RequestInit) => {
    sent.push(JSON.parse(String(init.body)));
    return new Response(JSON.stringify(body), { status, headers });
  });
  return sent;
}

// fillNew fills every required field of a new preset.
function fillNew() {
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Fast PLA for the P1S" } });
  fireEvent.change(screen.getByLabelText("Printer"), { target: { value: "bambu-lab-p1s" } });
  fireEvent.change(screen.getByLabelText("Filament"), { target: { value: "bambu-lab-pla-basic" } });
  fireEvent.change(screen.getByLabelText("Slicer"), { target: { value: "orcaslicer" } });
}

test("a new preset starts with a 0.4 mm nozzle, the CC BY 4.0 license and no other choice made", () => {
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);

  expect((screen.getByLabelText("Printer") as HTMLSelectElement).value).toBe("");
  expect((screen.getByLabelText("Nozzle") as HTMLSelectElement).value).toBe("0.4");
  expect((screen.getByLabelText(/^CC BY 4\.0/) as HTMLInputElement).checked).toBe(true);
});

test("a missing field shows its message under the field and takes the focus, without a call to the API", async () => {
  const sent = stubAPI(201, preset);
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Fast PLA for the P1S" } });

  fireEvent.click(screen.getByRole("button", { name: "Publish" }));

  const printer = screen.getByLabelText("Printer");
  expect(await screen.findByText("Choose a printer.")).toBeDefined();
  expect(printer.getAttribute("aria-invalid")).toBe("true");
  expect(document.activeElement).toBe(printer);
  expect(sent).toHaveLength(0);
});

test("Publish and Save as draft send the whole preset with the status", async () => {
  const sent = stubAPI(201, preset);
  const onSaved = vi.fn();
  render(<PresetForm catalog={catalog} onSaved={onSaved} />);
  fillNew();

  fireEvent.click(screen.getByRole("button", { name: "Save as draft" }));

  await waitFor(() => expect(onSaved).toHaveBeenCalledWith(preset));
  expect(sent[0]).toEqual({
    title: "Fast PLA for the P1S",
    description: "",
    printer: "bambu-lab-p1s",
    material: "bambu-lab-pla-basic",
    nozzle_mm: 0.4,
    slicer: "orcaslicer",
    slicer_version: "",
    license: "CC-BY-4.0",
    status: "draft",
  });
});

test("a message from the API goes under the field that it names", async () => {
  stubAPI(400, { detail: "printer is not in the catalog" });
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  fillNew();

  fireEvent.click(screen.getByRole("button", { name: "Publish" }));

  expect(await screen.findByText("printer is not in the catalog")).toBeDefined();
  expect(screen.getByLabelText("Printer").getAttribute("aria-invalid")).toBe("true");
});

test("editing fills in the preset and keeps a draft a draft until the author publishes it", async () => {
  const sent = stubAPI(200, preset);
  render(<PresetForm catalog={catalog} preset={preset} onSaved={() => {}} />);

  expect((screen.getByLabelText("Title") as HTMLInputElement).value).toBe("Fast PLA for the P1S");
  expect((screen.getByLabelText("Nozzle") as HTMLSelectElement).value).toBe("0.6");
  expect((screen.getByLabelText(/^CC0 1\.0/) as HTMLInputElement).checked).toBe(true);

  fireEvent.click(screen.getByRole("button", { name: "Save as draft" }));
  await waitFor(() => expect(sent).toHaveLength(1));
  expect(sent[0]).toMatchObject({ status: "draft", nozzle_mm: 0.6 });
});

test("at the create limit the form says when to try again, above the buttons", async () => {
  stubAPI(429, { detail: "too many requests" }, { "Retry-After": "600" });
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  fillNew();

  fireEvent.click(screen.getByRole("button", { name: "Publish" }));

  expect((await screen.findByRole("alert")).textContent).toBe("You shared many presets in the last hour. Try again in 10 minutes.");
});

test("near the limit the description counts the characters left, in the singular for one", () => {
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  const description = screen.getByLabelText("Description (optional)");

  fireEvent.change(description, { target: { value: "x".repeat(3999) } });
  expect(screen.getByText("1 character left.")).toBeDefined();
  fireEvent.change(description, { target: { value: "x".repeat(3600) } });
  expect(screen.getByText("400 characters left.")).toBeDefined();
});

test("the new form keeps what was typed in this tab, and Start over clears it", () => {
  const { unmount } = render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Half typed" } });
  unmount();

  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  expect((screen.getByLabelText("Title") as HTMLInputElement).value).toBe("Half typed");
  fireEvent.click(screen.getByRole("button", { name: "Start over" }));
  expect((screen.getByLabelText("Title") as HTMLInputElement).value).toBe("");
});

test("an empty form puts the focus on the printer, the first field of the page", () => {
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);

  fireEvent.click(screen.getByRole("button", { name: "Publish" }));

  expect(document.activeElement).toBe(screen.getByLabelText("Printer"));
  expect(screen.getByText("Give the preset a title.")).toBeDefined();
});

test("the edit form links back to the preset", () => {
  render(<PresetForm catalog={catalog} preset={preset} cancelHref="/presets/fast-pla-ab3defgh" onSaved={() => {}} />);

  expect(screen.getByRole("link", { name: "Cancel" })).toHaveProperty("href", expect.stringMatching(/\/presets\/fast-pla-ab3defgh$/));
});

// stubUploadAPI answers the upload steps with a checked PrusaSlicer filament file (or a refusal), and keeps the
// preset bodies that the form sends.
function stubUploadAPI(complete: { status: number; body: unknown }) {
  const presets: unknown[] = [];
  vi.stubGlobal("fetch", async (url: string, init: RequestInit) => {
    const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });
    if (url.endsWith("/api/v1/uploads")) {
      return json(201, { id: "abcdefghjkmnpqrs", url: "https://bucket.test/u/x", method: "PUT", headers: { "Content-Type": "application/octet-stream" } });
    }
    if (url.startsWith("https://bucket.test/")) return new Response(null, { status: 200 });
    if (url.endsWith("/complete")) return json(complete.status, complete.body);
    presets.push(JSON.parse(String(init.body)));
    return json(201, preset);
  });
  return presets;
}

const checked = {
  id: "abcdefghjkmnpqrs",
  status: "ready",
  slicer: "prusaslicer",
  kind: "filament",
  slicer_version: "2.9.6",
  settings: [{ id: "nozzle_temperature", label: "Nozzle temperature", value: "210", unit: "°C" }],
  removed: ["printhost_password"],
};

test("a chosen file is checked and read back, fills in the slicer, and goes with the preset", async () => {
  const presets = stubUploadAPI({ status: 200, body: checked });
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);

  fireEvent.change(screen.getByLabelText("Preset file (optional)"), { target: { files: [new File(["x"], "my.ini")] } });

  expect(await screen.findByText("Nozzle temperature")).toBeDefined();
  expect(screen.getByText("210 °C")).toBeDefined();
  expect(screen.getByText(/^PrusaSlicer filament settings, version 2\.9\.6$/)).toBeDefined();
  expect(screen.getAllByText(/the printer's password/)).toHaveLength(2); // the panel note and the status line
  expect((screen.getByLabelText("Slicer") as HTMLSelectElement).value).toBe("prusaslicer");
  expect((screen.getByLabelText("Slicer version (optional)") as HTMLInputElement).value).toBe("2.9.6");
  // One status line, always there, tells a screen reader what happened.
  expect(screen.getByRole("status").textContent).toBe(
    "Checked: PrusaSlicer filament settings, version 2.9.6. Filled in from the file: Slicer, Slicer version. " +
      "The file held the printer's password, so it was removed before the file was stored. Downloads never include it.",
  );

  fillNew();
  fireEvent.change(screen.getByLabelText("Slicer"), { target: { value: "prusaslicer" } });
  fireEvent.click(screen.getByRole("button", { name: "Publish" }));
  await waitFor(() => expect(presets).toHaveLength(1));
  expect(presets[0]).toMatchObject({ file: "abcdefghjkmnpqrs", slicer: "prusaslicer" });
});

test("a refused file shows the reason under the file field", async () => {
  stubUploadAPI({ status: 422, body: { detail: "the file is not a preset from PrusaSlicer, SuperSlicer, OrcaSlicer or Bambu Studio" } });
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  const input = screen.getByLabelText("Preset file (optional)");

  fireEvent.change(input, { target: { files: [new File(["<html>"], "page.html")] } });

  expect(await screen.findByText(/^The file is not a preset from PrusaSlicer.*\. Export the preset from the slicer/)).toBeDefined();
  expect(input.getAttribute("aria-invalid")).toBe("true");
  expect(screen.getByRole("status").textContent).toMatch(/^The file was refused. The file is not a preset/);
});

test("a wrong second file keeps the checked first one, and Remove file clears it", async () => {
  let calls = 0;
  vi.stubGlobal("fetch", async (url: string) => {
    const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });
    if (url.endsWith("/api/v1/uploads")) return json(201, { id: "abcdefghjkmnpqrs", url: "https://bucket.test/u/x", method: "PUT", headers: {} });
    if (url.startsWith("https://bucket.test/")) return new Response(null, { status: 200 });
    calls++;
    return calls === 1 ? json(200, checked) : json(422, { detail: "the file is not a preset" });
  });
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  const input = screen.getByLabelText("Preset file (optional)");

  fireEvent.change(input, { target: { files: [new File(["x"], "good.ini")] } });
  expect(await screen.findByText("Nozzle temperature")).toBeDefined();
  fireEvent.change(input, { target: { files: [new File(["<html>"], "bad.html")] } });
  expect(await screen.findByText(/^The file is not a preset/)).toBeDefined();
  expect(screen.getByText("Nozzle temperature")).toBeDefined();

  fireEvent.click(screen.getByRole("button", { name: "Remove file" }));
  expect(screen.queryByText("Nozzle temperature")).toBeNull();
});

test("while a file is checked, the field keeps the focus and stays usable", async () => {
  let finish: (r: Response) => void = () => {};
  vi.stubGlobal("fetch", async (url: string) => {
    if (url.endsWith("/api/v1/uploads")) {
      return new Response(JSON.stringify({ id: "abcdefghjkmnpqrs", url: "https://bucket.test/u/x", method: "PUT", headers: {} }), { status: 201 });
    }
    if (url.startsWith("https://bucket.test/")) return new Response(null, { status: 200 });
    return new Promise<Response>((resolve) => (finish = resolve));
  });
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  const input = screen.getByLabelText("Preset file (optional)") as HTMLInputElement;
  input.focus();

  fireEvent.change(input, { target: { files: [new File(["x"], "my.ini")] } });
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Checking the file…"));
  expect(input.disabled).toBe(false);
  expect(document.activeElement).toBe(input);
  finish(new Response(JSON.stringify(checked), { status: 200 }));
  expect(await screen.findByText("Nozzle temperature")).toBeDefined();
});

test("an empty or too large file gets a plain reason without a call to the API", () => {
  const sent = stubAPI(201, preset);
  render(<PresetForm catalog={catalog} onSaved={() => {}} />);
  const input = screen.getByLabelText("Preset file (optional)");

  fireEvent.change(input, { target: { files: [new File([], "empty.ini")] } });
  expect(screen.getByText("The file is empty.")).toBeDefined();
  fireEvent.change(input, { target: { files: [new File([new Uint8Array((1 << 20) + 1)], "big.3mf")] } });
  expect(screen.getByText("The file is larger than 1 MB, so it is not a preset file.")).toBeDefined();
  expect(sent).toHaveLength(0);
});
