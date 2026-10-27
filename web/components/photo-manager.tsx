"use client";

import { type FormEvent, useEffect, useId, useRef, useState } from "react";
import { ApiError, waitText } from "../lib/account";
import type { Photo } from "../lib/api-types";
import { addPhoto, removePhoto } from "../lib/my-presets";
import { Notice } from "./account-ui";
import { thumbSize } from "./photo-gallery";

const maxPhotos = 10;
const maxPhotoBytes = 10 * 1024 * 1024;
const maxAlt = 200;

// problemText says why a photo was not added, from the API's reason when there is one.
function problemText(err: unknown): string {
  if (err instanceof ApiError && err.status === 429) return `You added many photos in the last hour. Try again ${waitText(err.retryAfter)}.`;
  if (err instanceof ApiError && (err.status === 422 || err.status === 409) && err.detail) {
    const sentence = err.detail.charAt(0).toUpperCase() + err.detail.slice(1);
    return sentence.endsWith(".") ? sentence : `${sentence}.`;
  }
  return "The photo was not added. Try again.";
}

// PhotoItem is one photo of the list with Remove and its in-place confirm. PhotoManager renders one per photo.
function PhotoItem({ photo, onRemove }: { photo: Photo; onRemove: () => Promise<void> }) {
  const [confirm, setConfirm] = useState(false);
  const [failed, setFailed] = useState(false);

  async function onConfirm() {
    setFailed(false);
    try {
      await onRemove();
    } catch {
      setFailed(true);
    }
  }

  return (
    <li className="flex min-w-0 flex-col gap-2">
      {/* No link here: a click would leave the edit page and lose changes that are not saved yet. */}
      <div className="photo-thumb">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src={photo.thumb_url} alt={photo.alt} {...thumbSize(photo)} loading="lazy" decoding="async" />
      </div>
      <p dir="auto" className="text-sm wrap-anywhere">
        {photo.alt}
      </p>
      {confirm ? (
        <div role="group" aria-label="Confirm the removal" className="flex flex-col gap-2">
          <p className="text-sm">Remove this photo?</p>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn btn-danger" onClick={onConfirm}>
              Remove for good
            </button>
            <button type="button" className="btn btn-secondary" onClick={() => setConfirm(false)} autoFocus>
              Cancel
            </button>
          </div>
        </div>
      ) : (
        <button type="button" className="btn btn-secondary self-start" onClick={() => setConfirm(true)}>
          {/* The space stays outside the hidden span: a name computed from the span alone drops it. */}
          Remove <span className="sr-only">photo: {photo.alt}</span>
        </button>
      )}
      {failed && <Notice tone="error">The photo was not removed. Try again.</Notice>}
    </li>
  );
}

// PhotoManager is the Photos section of the edit page: the form that adds a photo with its description, and the
// photos with Remove. Each change goes to the API at once, apart from the preset form's Save.
export function PhotoManager({ presetId, initial }: { presetId: string; initial: Photo[] }) {
  const id = useId();
  const [photos, setPhotos] = useState(initial);
  const [file, setFile] = useState<File | null>(null);
  const [alt, setAlt] = useState("");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<{ field: "file" | "alt" | "api"; text: string } | null>(null);
  const [note, setNote] = useState("");
  const heading = useRef<HTMLHeadingElement>(null);
  const picker = useRef<HTMLInputElement>(null);
  const altInput = useRef<HTMLInputElement>(null);

  // A link to "#photos", as in the notice after a publish, lands here: the section renders after the member loads,
  // so the browser cannot scroll to it by itself. Focus scrolls the heading into view and puts the keyboard there.
  useEffect(() => {
    if (location.hash === "#photos") heading.current?.focus();
  }, []);

  // refuse shows a problem with one field and takes the focus to that field.
  function refuse(field: "file" | "alt", text: string) {
    setProblem({ field, text });
    (field === "file" ? picker : altInput).current?.focus();
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setNote("");
    if (!file) return refuse("file", "Choose a photo first.");
    if (file.size > maxPhotoBytes) return refuse("file", "The photo is larger than 10 MB. Save a smaller copy and try again.");
    if (alt.trim() === "") return refuse("alt", "Describe the photo for people who cannot see it.");
    setProblem(null);
    setBusy(true);
    setNote("Checking the photo…");
    try {
      const added = await addPhoto(presetId, file, alt.trim());
      setPhotos((now) => [...now, added]);
      setFile(null);
      setAlt("");
      if (picker.current) picker.current.value = "";
      setNote("Photo added.");
    } catch (err) {
      setNote("");
      setProblem({ field: "api", text: problemText(err) });
    } finally {
      setBusy(false);
    }
  }

  async function remove(photoId: string) {
    await removePhoto(photoId);
    // The pressed button leaves with its photo, so the focus goes to the section heading.
    heading.current?.focus();
    setPhotos((now) => now.filter((p) => p.id !== photoId));
    setNote("Photo removed.");
  }

  return (
    <section id="photos" aria-labelledby={`${id}-heading`} className="mt-12 max-w-3xl scroll-mt-6">
      <h2 id={`${id}-heading`} ref={heading} tabIndex={-1} className="text-lg font-semibold focus:outline-none">
        Photos
      </h2>
      <p className="mt-1 max-w-prose text-sm text-muted">
        A photo of a print made with this preset marks it Photo verified. Photos show in the order you add them, and they
        save at once: they do not wait for Save.
      </p>
      {/* The status line stays in the page, so a screen reader reads each new note. */}
      <p role="status" className="mt-3 text-sm text-muted empty:hidden">
        {note}
      </p>

      {photos.length < maxPhotos ? (
        <form onSubmit={onSubmit} noValidate className="panel mt-4 flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label htmlFor={`${id}-file`} className="text-sm font-medium">
              Photo (JPEG or PNG, up to 10 MB)
            </label>
            <input
              ref={picker}
              id={`${id}-file`}
              type="file"
              accept="image/jpeg,image/png"
              className="file-input"
              aria-invalid={problem?.field === "file" || undefined}
              aria-describedby={problem?.field === "file" ? `${id}-problem` : undefined}
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor={`${id}-alt`} className="text-sm font-medium">
              Describe the photo
            </label>
            <input
              ref={altInput}
              id={`${id}-alt`}
              className="field"
              value={alt}
              maxLength={maxAlt}
              onChange={(e) => setAlt(e.target.value)}
              aria-invalid={problem?.field === "alt" || undefined}
              aria-describedby={problem?.field === "alt" ? `${id}-alt-hint ${id}-problem` : `${id}-alt-hint`}
            />
            <p id={`${id}-alt-hint`} className="text-sm text-muted">
              What the print shows, for people who cannot see it. For example: the benchy from the side, with clean overhangs.
            </p>
          </div>
          {problem && (
            <div id={`${id}-problem`}>
              <Notice tone="error">{problem.text}</Notice>
            </div>
          )}
          <div>
            {/* Not disabled while busy: a disabled button drops the focus to the page. onSubmit ignores a second press. */}
            <button type="submit" className="btn btn-primary" aria-disabled={busy || undefined}>
              {busy ? "Adding…" : "Add photo"}
            </button>
          </div>
        </form>
      ) : (
        <p className="mt-4 text-sm text-muted">This preset has 10 photos, the most it can have. Remove one to add another.</p>
      )}

      {photos.length === 0 ? (
        <p className="mt-6 text-sm text-muted">No photos yet. Add one of a print that you made with this preset.</p>
      ) : (
        <p className="mt-6 text-sm text-muted">
          {photos.length} of {maxPhotos} photos
        </p>
      )}
      {photos.length > 0 && (
        <ul className="mt-3 grid grid-cols-2 gap-4 sm:grid-cols-3">
          {photos.map((p) => (
            <PhotoItem key={p.id} photo={p} onRemove={() => remove(p.id)} />
          ))}
        </ul>
      )}
    </section>
  );
}
