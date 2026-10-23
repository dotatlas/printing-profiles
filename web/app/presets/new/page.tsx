import type { Metadata } from "next";
import { connection } from "next/server";
import { Suspense } from "react";
import { NewPreset } from "../../../components/preset-pages";
import { loadCatalog } from "../../../lib/api";

export const metadata: Metadata = { title: "Share a preset", robots: { index: false } };

// FormWithCatalog gets the catalog for the selects on each request, then shows the form. NewPresetPage streams it.
async function FormWithCatalog() {
  await connection();
  return <NewPreset catalog={await loadCatalog()} />;
}

// NewPresetPage is the route /presets/new. The heading is in the server HTML; the form needs a signed-in member.
export default function NewPresetPage() {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Share a preset</h1>
      <p className="mt-2 max-w-prose text-muted">
        Add your preset file, then the printer, filament, nozzle and slicer that it is for.
      </p>
      <div className="mt-8">
        <Suspense>
          <FormWithCatalog />
        </Suspense>
      </div>
    </>
  );
}
