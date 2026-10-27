import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import type { Preset, Printer, Slicer } from "../lib/api-types";
import { PresetDetails, PresetFacts } from "./preset-facts";
import { PresetView } from "./preset-view";

const bambu = { slug: "bambu-lab", name: "Bambu Lab" };
const printers: Printer[] = [
  { slug: "bambu-lab-p1s", model: "P1S", brand: bambu },
  { slug: "prusa-research-mk4s", model: "MK4S", brand: { slug: "prusa-research", name: "Prusa Research" } },
];
const slicers: Slicer[] = [{ slug: "orcaslicer", name: "OrcaSlicer" }];

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
  verified: false,
  published_at: "2026-10-20T10:00:00Z",
};

test("each fact of a preset links to the browse page for that fact, and one link keeps the whole setup", () => {
  render(<PresetFacts preset={preset} />);

  expect(screen.getByRole("link", { name: "More presets for the Printer Bambu Lab P1S" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\?printer=bambu-lab-p1s$/),
  );
  expect(screen.getByRole("link", { name: "More presets for the Nozzle 0.4\u00a0mm" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\?nozzle=0\.4$/),
  );
  // The slicer tag carries the version, so the details panel does not repeat the slicer.
  expect(screen.getByRole("link", { name: "More presets for the Slicer OrcaSlicer 2.3.0" })).toBeDefined();
  expect(screen.getByRole("link", { name: "See every preset for this setup" })).toHaveProperty(
    "href",
    expect.stringMatching(/\/presets\?printer=bambu-lab-p1s&material=bambu-lab-pla-basic&nozzle=0\.4&slicer=orcaslicer$/),
  );
});

test("the details name the license with a link to its text and show the edit date when it differs", () => {
  render(<PresetDetails preset={preset} />);

  expect(screen.getByRole("link", { name: "CC BY-SA 4.0" })).toHaveProperty(
    "href",
    "https://creativecommons.org/licenses/by-sa/4.0/",
  );
  expect(screen.getByText("22 Oct 2026")).toBeDefined();
});

test("with a file, the preset page shows its key settings and a download that the API counts", () => {
  const file = {
    slicer: "prusaslicer" as const,
    kind: "filament" as const,
    slicer_version: "2.9.6",
    settings: [{ id: "nozzle_temperature", label: "Nozzle temperature", value: "210", unit: "°C" }],
    download_url: "https://api.test/api/v1/presets/ab3defgh/download",
    file_url: "https://files.test/presets/abc.ini",
  };
  const { unmount } = render(<PresetView preset={{ ...preset, file, download_count: 12 }} />);

  expect(screen.getByRole("heading", { name: "Filament settings in the file" })).toBeDefined();
  expect(screen.getByText("210 °C")).toBeDefined();
  expect(screen.getByRole("link", { name: "Download filament settings (.ini)" })).toHaveProperty("href", file.download_url);
  expect(screen.getByText(/PrusaSlicer 2\.9\.6/)).toBeDefined();
  expect(screen.getByText("12")).toBeDefined();
  // Download comes before the reviews and comments in the page, so the keyboard reaches it first on a phone too.
  const download = screen.getByRole("link", { name: "Download filament settings (.ini)" });
  const about = screen.getByRole("heading", { name: "About this preset" });
  expect(download.compareDocumentPosition(about) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  unmount();

  // A draft is not public, so its author downloads the file itself and nothing is counted.
  render(<PresetView preset={{ ...preset, status: "draft", file, download_count: 0 }} />);
  expect(screen.getByRole("link", { name: "Download filament settings (.ini)" })).toHaveProperty("href", file.file_url);
});

test("without a file, the preset page says so and offers no download", () => {
  render(<PresetView preset={preset} />);
  expect(screen.queryByRole("link", { name: /^Download/ })).toBeNull();
  expect(screen.getByText("None attached yet")).toBeDefined();
});

test("a photo-verified preset shows its photos and a badge that links to what it means", () => {
  const photos = [
    { id: "bcdefghjkmnp", url: "https://files.test/photos/a.jpg", thumb_url: "https://files.test/photos/a-thumb.jpg", width: 2048, height: 1536, alt: "The cube from the front" },
    { id: "cdefghjkmnpq", url: "https://files.test/photos/b.jpg", thumb_url: "https://files.test/photos/b-thumb.jpg", width: 1536, height: 2048, alt: "The seam on the back" },
  ];
  render(<PresetView preset={{ ...preset, verified: true, photos }} />);
  expect(screen.getByRole("link", { name: "Photo verified" }).getAttribute("href")).toBe("/faq#photo-verified");
  const first = screen.getByRole("img", { name: "The cube from the front" });
  expect(first.getAttribute("src")).toBe("https://files.test/photos/a-thumb.jpg");
  // The thumbnail is a link to the full image, and its size is set, so the page does not move while it loads.
  expect(first.closest("a")?.getAttribute("href")).toBe("https://files.test/photos/a.jpg");
  expect(first.getAttribute("width")).toBe("480");
  expect(first.getAttribute("height")).toBe("360");
  expect(screen.getByRole("img", { name: "The seam on the back" }).getAttribute("loading")).toBe("lazy");
});

test("a preset without photos has no badge and no photos section", () => {
  render(<PresetView preset={preset} />);
  expect(screen.queryByText("Photo verified")).toBeNull();
  expect(screen.queryByRole("heading", { name: "Photos" })).toBeNull();
});

test("the gallery shows six photos with their captions, keeps the rest behind Show more, and links to what the badge means", () => {
  const photos = Array.from({ length: 8 }, (_, i) => ({
    id: `bcdefghjkm${"abcdefgh"[i]}${"abcdefgh"[i]}`,
    url: `https://files.test/photos/${i}.jpg`,
    thumb_url: `https://files.test/photos/${i}-thumb.jpg`,
    width: 800,
    height: 600,
    alt: `Print ${i + 1}`,
  }));
  render(<PresetView preset={{ ...preset, verified: true, photos }} />);
  expect(screen.getByText("Print 1", { selector: "p" })).toBeDefined();
  const more = screen.getByText("Show 2 more photos");
  expect(more.closest("details")?.open).toBe(false);
  expect(more.closest("details")?.querySelectorAll("img")).toHaveLength(2);
  expect(screen.getByRole("link", { name: "What Photo verified means" }).getAttribute("href")).toBe("/faq#photo-verified");
});
