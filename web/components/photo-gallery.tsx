import Link from "next/link";
import type { Photo } from "../lib/api-types";

const thumbSide = 480; // the long side of a thumbnail, as the API makes it
const shownPhotos = 6; // the photos that the preset page shows before "Show more"

// thumbSize gives the size of a photo's thumbnail. The API never scales a photo up, so a small photo keeps its size.
export function thumbSize(p: Photo): { width: number; height: number } {
  const scale = Math.min(1, thumbSide / Math.max(p.width, p.height));
  return { width: Math.round(p.width * scale), height: Math.round(p.height * scale) };
}

// CheckMark is the small mark of the "Photo verified" tag.
function CheckMark() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" className="size-3.5 shrink-0">
      <circle cx="8" cy="8" r="6.5" fill="none" stroke="currentColor" strokeWidth="1.3" />
      <path d="m5.2 8.2 1.9 1.9 3.7-4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// VerifiedTag says that a preset is photo verified. On the preset page it links to what that means; in a browse
// row it is plain text, because the whole row is already a link.
export function VerifiedTag({ linked = false }: { linked?: boolean }) {
  if (!linked) {
    return (
      <span className="verified-tag">
        <CheckMark />
        Photo verified
      </span>
    );
  }
  return (
    <Link href="/faq#photo-verified" className="verified-tag">
      <CheckMark />
      Photo verified
    </Link>
  );
}

// PhotoThumbs is one grid of thumbnails with their captions; each thumbnail links to the full image. Each one has its
// size, so the page does not move while the photos load. PhotoGallery uses it.
function PhotoThumbs({ photos, eagerFirst }: { photos: Photo[]; eagerFirst: boolean }) {
  return (
    <ul className="mt-3 grid grid-cols-2 gap-x-3 gap-y-4 sm:grid-cols-3">
      {photos.map((p, i) => (
        <li key={p.id} className="flex min-w-0 flex-col gap-1.5">
          <a href={p.url} className="photo-thumb">
            {/* A plain img: the photo is already sized and re-encoded by the API, on the files origin. */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img src={p.thumb_url} alt={p.alt} {...thumbSize(p)} loading={eagerFirst && i === 0 ? "eager" : "lazy"} decoding="async" />
          </a>
          {/* The caption repeats the alt text for sighted readers; the image already says it to screen readers. */}
          <p aria-hidden="true" dir="auto" className="line-clamp-2 text-[13px] text-muted wrap-anywhere">
            {p.alt}
          </p>
        </li>
      ))}
    </ul>
  );
}

// PhotoGallery shows a preset's photos, oldest first: the first six, then the rest behind "Show more", so many photos
// never bury the preset. The preset page renders it on the server.
export function PhotoGallery({ photos }: { photos: Photo[] }) {
  const rest = photos.slice(shownPhotos);
  return (
    <section aria-labelledby="photos">
      <h2 id="photos" className="font-semibold">
        Photos
      </h2>
      <p className="mt-1 text-sm text-muted">
        Prints that the author made with this preset.{" "}
        <Link href="/faq#photo-verified">What Photo verified means</Link>
      </p>
      <PhotoThumbs photos={photos.slice(0, shownPhotos)} eagerFirst />
      {rest.length > 0 && (
        <details className="mt-4">
          <summary className="cursor-pointer text-sm font-medium text-accent">
            Show {rest.length} more {rest.length === 1 ? "photo" : "photos"}
          </summary>
          <PhotoThumbs photos={rest} eagerFirst={false} />
        </details>
      )}
    </section>
  );
}
