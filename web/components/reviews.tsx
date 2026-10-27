"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { type FormEvent, useEffect, useId, useRef, useState } from "react";
import { ApiError, waitText, welcomePath } from "../lib/account";
import type { Review } from "../lib/api-types";
import { deleteMyReview, getMyReview, listReviews, saveReview } from "../lib/community";
import { formatDate, tidyText } from "../lib/format";
import { profilePath } from "../lib/presets";
import { Notice } from "./account-ui";
import { useSession } from "./session";
import { ShowMore } from "./show-more";
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

// ReviewForm makes or changes the member's review, from start when there is one. onSaved gets the saved review;
// onCancel closes the form. MyReview opens and closes it.
function ReviewForm({
  presetId,
  start,
  onSaved,
  onCancel,
}: {
  presetId: string;
  start: Review | null;
  onSaved: (review: Review) => void;
  onCancel: () => void;
}) {
  const id = useId();
  const [stars, setStars] = useState(start?.stars ?? 0);
  const [body, setBody] = useState(start?.body ?? "");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (stars < 1) {
      setProblem("Choose 1 to 5 stars.");
      return;
    }
    setBusy(true);
    setProblem("");
    try {
      onSaved(await saveReview(presetId, stars, body.trim()));
    } catch (err) {
      setProblem(
        err instanceof ApiError && err.status === 429
          ? `You posted many reviews in the last hour. Try again ${waitText(err.retryAfter)}.`
          : "The review was not saved. Try again.",
      );
      setBusy(false);
    }
  }

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
      <div className="flex flex-wrap gap-3">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {busy ? "Saving…" : start ? "Update review" : "Post review"}
        </button>
        <button type="button" className="btn btn-secondary" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  );
}

// ReviewBody shows one review: the stars, the reviewer, the date and the text. The list and MyReview use it.
function ReviewBody({ review }: { review: Review }) {
  return (
    <>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <Stars value={review.stars} />
        <Link href={profilePath(review.author.handle)} className="text-sm font-medium text-ink no-underline wrap-anywhere hover:underline">
          {review.author.display_name || `@${review.author.handle}`}
        </Link>
        <span className="text-sm text-muted">
          {formatDate(review.created_at)}
          {review.edited && " · edited"}
        </span>
      </div>
      {review.body && <p className="mt-2 max-w-prose whitespace-pre-line wrap-anywhere">{tidyText(review.body)}</p>}
    </>
  );
}

// MyReview is the signed-in member's part of the reviews section: "Write a review", the form, or the member's own
// review with Edit and Remove. It calls onChange after each save or removal, so the list and the rating follow.
function MyReview({ presetId, onChange }: { presetId: string; onChange: () => void }) {
  const [mine, setMine] = useState<Review | null | undefined>(undefined);
  const [mode, setMode] = useState<"read" | "form" | "confirm">("read");
  const [note, setNote] = useState("");
  const [problem, setProblem] = useState("");
  const focus = useRef<"mine" | "write" | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const write = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    let live = true;
    getMyReview(presetId).then(
      (r) => live && setMine(r),
      () => live && setMine(null),
    );
    return () => {
      live = false;
    };
  }, [presetId]);

  // The pressed button leaves with the form or the confirm, so after that render the focus goes to what takes its
  // place.
  useEffect(() => {
    if (focus.current === "mine") heading.current?.focus();
    if (focus.current === "write") write.current?.focus();
    focus.current = null;
  });

  function onSaved(review: Review) {
    setNote(mine ? "Your review is updated." : "Your review is posted.");
    setMine(review);
    setMode("read");
    focus.current = "mine";
    onChange();
  }

  async function onRemove() {
    setProblem("");
    try {
      await deleteMyReview(presetId);
      setMine(null);
      setMode("read");
      setNote("Your review is removed.");
      focus.current = "write";
      onChange();
    } catch {
      setProblem("The review was not removed. Try again.");
    }
  }

  let body;
  // The space of the button stays while the member's review loads, so the list does not jump.
  if (mine === undefined) body = <div className="h-10" aria-hidden="true" />;
  else if (mode === "form") {
    body = <ReviewForm presetId={presetId} start={mine} onSaved={onSaved} onCancel={() => setMode("read")} />;
  } else if (!mine) {
    body = (
      <button ref={write} type="button" className="btn btn-secondary" onClick={() => setMode("form")}>
        Write a review
      </button>
    );
  } else {
    body = (
      <section aria-labelledby={`mine-${presetId}`} className="panel max-w-2xl">
        <h3 id={`mine-${presetId}`} ref={heading} tabIndex={-1} className="mb-2 text-sm font-medium focus:outline-none">
          Your review
        </h3>
        <ReviewBody review={mine} />
        {mode === "confirm" ? (
          <div role="group" aria-label="Confirm the removal" className="mt-4 flex flex-col gap-2">
            <p className="text-sm">Remove your review?</p>
            <div className="flex flex-wrap gap-3">
              <button type="button" className="btn btn-danger" onClick={onRemove}>
                Remove for good
              </button>
              <button type="button" className="btn btn-secondary" onClick={() => setMode("read")} autoFocus>
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div className="mt-4 flex flex-wrap gap-3">
            <button type="button" className="btn btn-secondary" onClick={() => setMode("form")}>
              Edit review
            </button>
            <button type="button" className="btn btn-secondary" onClick={() => setMode("confirm")}>
              Remove review
            </button>
          </div>
        )}
        {problem && (
          <div className="mt-3">
            <Notice tone="error">{problem}</Notice>
          </div>
        )}
      </section>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {/* The status line stays in the page, so a screen reader reads each new note. */}
      <p role="status" className="text-sm text-muted empty:hidden">
        {note}
      </p>
      {body}
    </div>
  );
}

// Reviews is the reviews section of the preset page: the member's own part (or why there is none), then the other
// reviews, newest first. The server gives the first page and the cursor of the next one; "Show more reviews" reads
// the next pages, and the section reads the first page again after each change.
export function Reviews({
  presetId,
  authorHandle,
  initial,
  next,
}: {
  presetId: string;
  authorHandle: string;
  initial: Review[];
  next?: string;
}) {
  const { state } = useSession();
  const pathname = usePathname();
  const router = useRouter();
  const [items, setItems] = useState(initial);
  const [cursor, setCursor] = useState(next);
  const [moreFailed, setMoreFailed] = useState(false);
  const me = state.status === "signed-in" && !state.me.needs_onboarding ? state.me : null;

  // reload reads the list again, and refreshes the server parts of the page, such as the rating under the title.
  // The member's review is new or changed, so it is on the first page.
  async function reload() {
    router.refresh();
    try {
      const page = await listReviews(presetId);
      setItems(page.items);
      setCursor(page.next_cursor);
    } catch {
      // The list stays as it was; the next page load shows the change.
    }
  }

  // showMore adds the next page of reviews under the reviews on screen.
  async function showMore() {
    setMoreFailed(false);
    try {
      const page = await listReviews(presetId, cursor);
      setItems((now) => [...now, ...page.items]);
      setCursor(page.next_cursor);
    } catch {
      setMoreFailed(true);
    }
  }

  let own = null;
  if (state.status === "signed-out") {
    own = (
      <p>
        <Link href={`/signin?return=${encodeURIComponent(`${pathname}#reviews`)}`}>Sign in to review</Link>
      </p>
    );
  } else if (state.status === "signed-in" && state.me.handle === authorHandle) {
    own = <p className="text-muted">Reviews come from people who printed your preset.</p>;
  } else if (state.status === "signed-in" && state.me.needs_onboarding) {
    own = (
      <p>
        <Link href={welcomePath(`${pathname}#reviews`)}>Choose your handle to review</Link>
      </p>
    );
  } else if (me) {
    own = <MyReview presetId={presetId} onChange={reload} />;
  }
  // The member's own review shows once, in MyReview.
  const others = me ? items.filter((r) => r.author.handle !== me.handle) : items;

  return (
    <section aria-labelledby="reviews" className="scroll-mt-6">
      <h2 id="reviews" className="font-semibold">
        Reviews
      </h2>
      <div className="mt-3">{own}</div>
      {others.length === 0 ? (
        <p className="mt-4 text-muted">{items.length === 0 ? "No reviews yet. If you tried it, say how it printed." : "No other reviews yet."}</p>
      ) : (
        <ul className="mt-4 divide-y divide-line border-t border-line">
          {others.map((r) => (
            <li key={`${r.author.handle}-${r.created_at}`} className="py-4">
              <ReviewBody review={r} />
            </li>
          ))}
        </ul>
      )}
      {cursor && <ShowMore label="Show more reviews" onMore={showMore} failed={moreFailed} />}
    </section>
  );
}
