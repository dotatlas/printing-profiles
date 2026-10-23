import { connection } from "next/server";
import Link from "next/link";
import { Suspense } from "react";
import { Catalog, CatalogSkeleton } from "../components/catalog";
import { listFirst } from "../lib/api";

// CatalogFromAPI gets the three catalog lists from the API for each request. HomePage streams it in <Suspense>.
async function CatalogFromAPI() {
  // Prerendering stops here, so the build never needs API_URL or calls the API.
  await connection();
  const [printers, materials, slicers] = await Promise.all([
    listFirst("/api/v1/printers"),
    listFirst("/api/v1/materials"),
    listFirst("/api/v1/slicers"),
  ]);
  return <Catalog printers={printers} materials={materials} slicers={slicers} />;
}

// HomePage is the start page of the site. Next.js renders it for "/": the build prerenders the intro, and the catalog streams in.
export default function HomePage() {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">Slicer presets for your printer and filament</h1>
      <p className="mt-3 max-w-prose text-muted">
        Find settings that print well on your exact printer, with your filament and your slicer.
      </p>
      <p className="mt-6">
        <Link href="/presets" className="btn btn-primary">
          Browse presets
        </Link>
      </p>
      <Suspense fallback={<CatalogSkeleton />}>
        <CatalogFromAPI />
      </Suspense>
    </>
  );
}
