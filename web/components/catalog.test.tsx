import { render, screen, within } from "@testing-library/react";
import { expect, test } from "vitest";
import type { Material, Printer } from "../lib/api-types";
import { Catalog } from "./catalog";

const bambu = { slug: "bambu-lab", name: "Bambu Lab" };
const printers: Printer[] = [
  { slug: "bambu-lab-a1", model: "A1", brand: bambu },
  { slug: "bambu-lab-p1s", model: "P1S", brand: bambu },
  { slug: "prusa-research-mk4s", model: "MK4S", brand: { slug: "prusa-research", name: "Prusa Research" } },
];
// The API sorts by slug, so PETG from eSun comes before PLA from Overture.
const materials: Material[] = [
  { slug: "esun-petg", product_name: "PETG", type: "PETG", diameter_mm: 1.75, brand: { slug: "esun", name: "eSun" }, colors: [] },
  {
    slug: "overture-pla",
    product_name: "PLA",
    type: "PLA",
    diameter_mm: 1.75,
    brand: { slug: "overture", name: "Overture" },
    colors: [
      { name: "Black", hex: "#000000" },
      { name: "White", hex: "#ffffff" },
    ],
  },
];

// itemsOf gives the texts of the items in the row of a group, for example the models of one brand.
function itemsOf(group: string) {
  const row = screen.getByText(group, { selector: "dt" }).nextElementSibling as HTMLElement;
  return within(row).getAllByRole("listitem").map((item) => item.textContent);
}

test("printers are grouped by brand, and filaments by type with PLA first", () => {
  render(<Catalog printers={printers} materials={materials} slicers={[{ slug: "orcaslicer", name: "OrcaSlicer" }]} />);

  expect(screen.getAllByRole("term").map((term) => term.textContent)).toEqual(["Bambu Lab", "Prusa Research", "PLA", "PETG"]);
  expect(itemsOf("Bambu Lab")).toEqual(["A1", "P1S"]);
  expect(screen.getByRole("heading", { name: "Printers 3" })).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Slicers 1" })).toBeTruthy();
});

test("a filament tells screen readers its color names", () => {
  render(<Catalog printers={[]} materials={materials} slicers={[]} />);

  expect(itemsOf("PLA")[0]).toContain("Overture PLA");
  expect(screen.getByText("(colors: Black, White)").className).toContain("sr-only");
  expect(itemsOf("PETG")).toEqual(["eSun PETG"]);
});

test("an empty list says so", () => {
  render(<Catalog printers={[]} materials={[]} slicers={[]} />);

  expect(screen.getByText("No printers yet.")).toBeTruthy();
  expect(screen.getByText("No filaments yet.")).toBeTruthy();
  expect(screen.getByText("No slicers yet.")).toBeTruthy();
});
