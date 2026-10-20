// HomePage is the start page of the site. Next.js renders it for "/".
export default function HomePage() {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">Slicer presets for your printer and filament</h1>
      <p className="mt-3 max-w-prose text-muted">
        Find settings that print well on your exact printer, with your filament and your slicer.
      </p>
    </>
  );
}
