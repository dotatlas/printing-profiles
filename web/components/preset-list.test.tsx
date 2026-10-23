import { render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import type { Material, Preset, Printer, Slicer } from "../lib/api-types";
import { BrowseFilters, NoMatches, PresetList } from "./preset-list";

const bambu = { slug: "bambu-lab", name: "Bambu Lab" };
const printers: Printer[] = [
  { slug: "bambu-lab-p1s", model: "P1S", brand: bambu },
  { slug: "prusa-research-mk4s", model: "MK4S", brand: { slug: "prusa-research", name: "Prusa Research" } },
];
const materials: Material[] = [
  { slug: "bambu-lab-pla-basic", product_name: "PLA Basic", type: "PLA", diameter_mm: 1.75, brand: bambu, colors: [] },
];
const slicers: Slicer[] = [{ slug: "orcaslicer", name: "OrcaSlicer" }];
const catalog = { printers, materials, slicers };

const preset: Preset = {
  id: "ab3defgh",
  slug: "fast-pla",
  title: "Fast PLA for the P1S",
  description: "First line.\nSecond line.",
  author: { handle: "maker", display_name: "Mia Maker" },
  printer: printers[0],
  material: { slug: "bambu-lab-pla-basic", product_name: "PLA Basic", type: "PLA", brand: bambu },
  nozzle_mm: 0.4,
  slicer: slicers[0],
  slicer_version: "2.3.0",
  license: "CC-BY-SA-4.0",
  status: "published",
  created_at: "2026-10-20T10:00:00Z",
  updated_at: "2026-10-22T10:00:00Z",
  download_count: 0,
  published_at: "2026-10-20T10:00:00Z",
};

test("with a filter in use, the rows mute the fact that the filter fixes", () => {
  render(<PresetList items={[preset]} query={{ printer: "bambu-lab-p1s" }} />);

  expect(screen.getByText("Bambu Lab P1S").closest("div")?.className).toContain("text-muted");
  expect(screen.getByText("Bambu Lab PLA Basic").closest("div")?.className).toContain("text-ink");
});

test("a row links to the preset and names each fact for screen readers", () => {
  render(<PresetList items={[preset]} />);
  const row = screen.getByRole("listitem");

  expect(within(row).getByRole("link", { name: "Fast PLA for the P1S" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\/fast-pla-ab3defgh$/),
  );
  expect(row.textContent).toContain("PrinterBambu Lab P1S");
  expect(row.textContent).toContain("FilamentBambu Lab PLA Basic");
  expect(row.textContent).toContain("Nozzle0.4\u00a0mm");
  expect(row.textContent).toContain("SlicerOrcaSlicer 2.3.0");
  expect(row.textContent).toContain("by Mia Maker · 20 Oct 2026");
});

test("a row names an author without a display name by the handle", () => {
  render(<PresetList items={[{ ...preset, author: { handle: "maker", display_name: "" } }]} />);

  expect(screen.getByRole("listitem").textContent).toContain("by @maker · 20 Oct 2026");
});

test("the filters show the current choice and mark the filters in use", () => {
  render(<BrowseFilters catalog={catalog} query={{ printer: "bambu-lab-p1s" }} />);

  const printer = screen.getByLabelText("Printer") as HTMLSelectElement;
  expect(printer.value).toBe("bambu-lab-p1s");
  expect(printer.dataset.active).toBe("true");
  expect((screen.getByLabelText("Slicer") as HTMLSelectElement).dataset.active).toBe("false");
  // The printers are grouped by brand, so a long list stays easy to scan.
  expect(within(printer).getByRole("group", { name: "Prusa Research" })).toBeDefined();
  expect(screen.getByRole("link", { name: "Clear filters" })).toHaveProperty("href", expect.stringMatching(/\/presets$/));
});

test("a filter value that the catalog does not know still shows, so the select does not claim Any", () => {
  render(<BrowseFilters catalog={catalog} query={{ printer: "old-printer" }} />);

  expect((screen.getByLabelText("Printer") as HTMLSelectElement).selectedOptions[0].textContent).toBe("old-printer");
});

test("with no match, each filter in use has a link that removes only that filter", () => {
  render(<NoMatches catalog={catalog} query={{ printer: "bambu-lab-p1s", nozzle: "0.6" }} />);

  expect(screen.getByRole("link", { name: "Remove Bambu Lab P1S" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\?nozzle=0\.6$/),
  );
  expect(screen.getByRole("link", { name: "Remove 0.6\u00a0mm" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\?printer=bambu-lab-p1s$/),
  );
});
