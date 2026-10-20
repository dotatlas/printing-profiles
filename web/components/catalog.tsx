import type { ReactNode } from "react";
import { type Color, type Material, materialTypeValues, type Printer, type Slicer } from "../lib/api-types";

// Section is one part of the catalog: a heading with the count, then the content, or a text when the list is empty.
function Section({ title, count, empty, children }: { title: string; count: number; empty: string; children: ReactNode }) {
  return (
    <section className="mt-10">
      <h2 className="flex items-baseline gap-2 border-b border-line pb-2 text-lg font-semibold">
        {title} <span className="text-sm font-normal text-muted">{count}</span>
      </h2>
      {count === 0 ? <p className="mt-3 text-muted">{empty}</p> : children}
    </section>
  );
}

// Row is one group of a list: the group name, then its items. On a phone, the name is above the items.
function Row({ name, children }: { name: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 py-3 sm:grid-cols-4 sm:gap-4">
      <dt className="font-medium">{name}</dt>
      <dd className="sm:col-span-3">
        <ul className="flex flex-wrap gap-x-4 gap-y-1">{children}</ul>
      </dd>
    </div>
  );
}

// Swatches shows a small circle for each color of a filament. Screen readers read the color names instead.
function Swatches({ colors }: { colors: Color[] }) {
  if (colors.length === 0) return null;
  return (
    <>
      <span className="sr-only">(colors: {colors.map((color) => color.name).join(", ")})</span>
      {colors.map((color) => (
        <svg key={color.name} viewBox="0 0 10 10" className="size-2.5" aria-hidden="true">
          <title>{color.name}</title>
          <circle cx="5" cy="5" r="4.5" fill={color.hex} stroke="currentColor" strokeOpacity="0.35" />
        </svg>
      ))}
    </>
  );
}

// Catalog shows the printers by brand, the filaments by type and the slicers. The home page gives it the API lists.
export function Catalog({ printers, materials, slicers }: { printers: Printer[]; materials: Material[]; slicers: Slicer[] }) {
  const printersByBrand = Map.groupBy(printers, (printer) => printer.brand.name);
  const materialsByType = Map.groupBy(materials, (material) => material.type);
  return (
    <>
      <Section title="Printers" count={printers.length} empty="No printers yet.">
        <dl className="divide-y divide-line">
          {[...printersByBrand].map(([brand, items]) => (
            <Row key={brand} name={brand}>
              {items.map((printer) => (
                <li key={printer.slug}>{printer.model}</li>
              ))}
            </Row>
          ))}
        </dl>
      </Section>
      <Section title="Filaments" count={materials.length} empty="No filaments yet.">
        <dl className="divide-y divide-line">
          {/* The types come in the order of the API contract, so the most common type (PLA) is first. */}
          {materialTypeValues.map((type) => {
            const items = materialsByType.get(type);
            return (
              items && (
                <Row key={type} name={type}>
                  {items.map((material) => (
                    <li key={material.slug} className="inline-flex items-center gap-1">
                      <span>
                        {material.brand.name} {material.product_name}
                      </span>
                      <Swatches colors={material.colors} />
                    </li>
                  ))}
                </Row>
              )
            );
          })}
        </dl>
      </Section>
      <Section title="Slicers" count={slicers.length} empty="No slicers yet.">
        <ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
          {slicers.map((slicer) => (
            <li key={slicer.slug}>{slicer.name}</li>
          ))}
        </ul>
      </Section>
    </>
  );
}

// CatalogSkeleton holds the place of the catalog while it loads. The home page shows it in <Suspense>.
export function CatalogSkeleton() {
  return (
    <div role="status" className="mt-10 space-y-10">
      <span className="sr-only">Loading the catalog</span>
      {["printers", "filaments", "slicers"].map((part) => (
        <div key={part} className="space-y-3" aria-hidden="true">
          <div className="h-6 w-32 rounded bg-line motion-safe:animate-pulse" />
          <div className="h-4 rounded bg-line motion-safe:animate-pulse" />
          <div className="h-4 w-2/3 rounded bg-line motion-safe:animate-pulse" />
        </div>
      ))}
    </div>
  );
}
