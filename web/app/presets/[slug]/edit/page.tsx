import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { EditPreset } from "../../../../components/preset-pages";
import { loadCatalog } from "../../../../lib/api";
import { parsePresetParam } from "../../../../lib/presets";

export const metadata: Metadata = { title: "Edit preset", robots: { index: false } };

// FormWithCatalog reads the preset ID from the path and the catalog from the API, then shows the edit form, which
// loads the preset in the browser with the session (a draft is visible only to its author).
async function FormWithCatalog({ params }: { params: PageProps<"/presets/[slug]/edit">["params"] }) {
  const parsed = parsePresetParam((await params).slug);
  if (!parsed) notFound();
  return <EditPreset catalog={await loadCatalog()} id={parsed.id} />;
}

// EditPresetPage is the route /presets/<slug>-<id>/edit. Only the author gets the form.
export default function EditPresetPage({ params }: PageProps<"/presets/[slug]/edit">) {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Edit preset</h1>
      <Suspense>
        <FormWithCatalog params={params} />
      </Suspense>
    </>
  );
}
