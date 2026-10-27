"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { type FormEvent, useEffect, useId, useState } from "react";
import { ApiError, waitText, welcomePath } from "../lib/account";
import type { Review } from "../lib/api-types";
import { deleteMyReview, getMyReview, listReviews, saveReview } from "../lib/community";
import { formatDate } from "../lib/format";
import { profilePath } from "../lib/presets";
import { Notice } from "./account-ui";
import { useSession } from "./session";
import { StarMark, Stars } from "./stars";

const maxReview = 2000;

// StarInput is the 1-5 rating of the review form: five native radios drawn as stars, so the arrow keys and screen
// readers work as with any radio group.
function StarInput({ value, onChange, invalid }: { value: number; onChange: (n: number) => void; invalid: boolean }) {
  const name = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <span id={`${name}-label`} className="text-sm font-medium">
        Your rating
      </span>
      <div role="radiogroup" aria-labelledby={`${name}-label`} aria-invalid={invalid || undefined} className="flex gap-1">
        {[1, 2, 3, 4, 5].map((n) => (
          <label key={n} className="star-choice">
            <input type="radio" name={name} value={n} checked={value === n} onChange={() => onChange(n)} className="sr-only" />
            <StarMark filled={n <= value} className="size-7" />
            <span className="sr-only">{n === 1 ? "1 star" : `${n} stars`}</span>
          </label>
        ))}
      </div>
    </div>
  );
}

// ReviewForm makes, changes or removes the signed-in member's review. It starts from the member's own review, and
// calls onChange after each save so the list shows it.
function ReviewForm({ presetId, onChange }: { presetId: string; onChange: () => void }) {
  const id = useId();
  const [ready, setReady] = useState(false);
  const [stars, setStars] = useState(0);
  const [body, setBody] = useState("");
  const [exists, setExists] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [note, setNote] = useState("");

  useEffect(() => {
    let live = true;
    getMyReview(presetId).then(
      (mine) => {
        if (!live) return;
        if (mine) {
          setStars(mine.stars);
          setBody(mine.body);
          setExists(true);
        }
        setReady(true);
      },
      () => live && setReady(true),
    );
    return () => {
      live = false;
    };
  }, [presetId]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setNote("");
    if (stars < 1) {
      setProblem("Choose 1 to 5 stars.");
      return;
    }
    setBusy(true);
    setProblem("");
    try {
      await saveReview(presetId, stars, body.trim());
      setNote(exists ? "Your review is updated." : "Your review is posted.");
      setExists(true);
      onChange();
    } catch (err) {
      setProblem(
        err instanceof ApiError && err.status === 429
          ? `You posted many reviews in the last hour. Try again ${waitText(err.retryAfter)}.`
          : "The review was not saved. Try again.",
      );
    } finally {
      setBusy(false);
    }
  }

  async function onRemove() {
    setBusy(true);
    setProblem("");
    try {
      await deleteMyReview(presetId);
      setStars(0);
      setBody("");
      setExists(false);
      setNote("Your review is removed.");
      onChange();
    } catch {
      setProblem("The review was not removed. Try again.");
    } finally {
      setBusy(false);
    }
  }

  if (!ready) return null;
  return (
    <form onSubmit={onSubmit} noValidate className="panel flex max-w-2xl flex-col gap-4">
      <StarInput value={stars} onChange={setStars} invalid={problem === "Choose 1 to 5 stars."} />
      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${id}-body`} className="text-sm font-medium">
          Your review (optional)
        </label>
        <textarea
          id={`${id}-body`}
          className="field h-auto min-h-24 py-2"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          maxLength={maxReview}
          aria-describedby={`${id}-hint`}
        />
        <p id={`${id}-hint`} className="text-sm text-muted">
          How did it print on your setup? Plain text.
        </p>
      </div>
      {problem && <Notice tone="error">{problem}</Notice>}
      {note && <Notice>{note}</Notice>}
      <div className="flex flex-wrap gap-3">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {exists ? "Update review" : "Post review"}
        </button>
        {exists && (
          <button type="button" className="btn btn-secondary" disabled={busy} onClick={onRemove}>
            Remove my review
          </button>
        )}
      </div>
    </form>
  );
}

// Reviews is the reviews section of the preset page: the member's review form (or why there is none), then the
// reviews, newest first. The server gives the first page; the section reads it again after each change.
export function Reviews({ presetId, authorHandle, initial }: { presetId: string; authorHandle: string; initial: Review[] }) {
  const { state } = useSession();
  const pathname = usePathname();
  const router = useRouter();
  const [items, setItems] = useState(initial);

  // reload reads the list again, and refreshes the server parts of the page, such as the rating under the title.
  async function reload() {
    router.refresh();
    try {
      setItems((await listReviews(presetId)).items);
    } catch {
      // The list stays as it was; the next page load shows the change.
    }
  }

  let form = null;
  if (state.status === "signed-out") {
    form = (
      <p>
        <Link href={`/signin?return=${encodeURIComponent(`${pathname}#reviews`)}`}>Sign in to review</Link>
      </p>
    );
  } else if (state.status === "signed-in" && state.me.handle === authorHandle) {
    form = <p className="text-muted">You cannot review your own preset.</p>;
  } else if (state.status === "signed-in" && state.me.needs_onboarding) {
    form = (
      <p>
        <Link href={welcomePath(`${pathname}#reviews`)}>Choose your handle to review</Link>
      </p>
    );
  } else if (state.status === "signed-in") {
    form = <ReviewForm presetId={presetId} onChange={reload} />;
  }

  return (
    <section aria-labelledby="reviews" className="scroll-mt-6">
      <h2 id="reviews" className="font-semibold">
        Reviews
      </h2>
      <div className="mt-3">{form}</div>
      {items.length === 0 ? (
        <p className="mt-4 text-muted">No reviews yet. If you tried it, say how it printed.</p>
      ) : (
        <ul className="mt-4 divide-y divide-line border-t border-line">
          {items.map((r) => (
            <li key={`${r.author.handle}-${r.created_at}`} className="py-4">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <Stars value={r.stars} />
                <Link href={profilePath(r.author.handle)} className="text-sm font-medium text-ink no-underline wrap-anywhere hover:underline">
                  {r.author.display_name || `@${r.author.handle}`}
                </Link>
                <span className="text-sm text-muted">
                  {formatDate(r.created_at)}
                  {r.edited && " · edited"}
                </span>
              </div>
              {r.body && <p className="mt-2 max-w-prose whitespace-pre-line wrap-anywhere">{r.body}</p>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
