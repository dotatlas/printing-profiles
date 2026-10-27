import type { Rating } from "../lib/api-types";
import { starsLabel } from "../lib/community";

// StarMark draws one star, filled or as an outline, in the current text color. Stars, the ratings and the star
// input of the review form use it.
export function StarMark({ filled, className = "size-4" }: { filled: boolean; className?: string }) {
  return (
    <svg viewBox="0 0 20 20" className={`shrink-0 ${className}`} aria-hidden="true">
      <path
        d="M10 1.8l2.5 5.2 5.7.8-4.1 4 1 5.7L10 14.8l-5.1 2.7 1-5.7-4.1-4 5.7-.8z"
        fill={filled ? "currentColor" : "none"}
        stroke="currentColor"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
    </svg>
  );
}

// Stars shows a review's whole-star rating as five marks; screen readers hear it in words.
export function Stars({ value }: { value: number }) {
  return (
    <span className="inline-flex items-center gap-0.5 text-accent">
      <span className="sr-only">{starsLabel(value)}</span>
      {[1, 2, 3, 4, 5].map((n) => (
        <StarMark key={n} filled={n <= value} />
      ))}
    </span>
  );
}

// reviewsText says a review count with its noun: "1 review", "12 reviews".
function reviewsText(count: number): string {
  return `${count} ${count === 1 ? "review" : "reviews"}`;
}

// RatingSummary shows a preset's average and review count under its title, as a link to the reviews. The preset
// page uses it.
export function RatingSummary({ rating }: { rating?: Rating }) {
  if (!rating) {
    return (
      <a href="#reviews" className="text-sm text-muted no-underline hover:underline">
        No reviews yet
      </a>
    );
  }
  return (
    <a href="#reviews" className="inline-flex items-center gap-1.5 text-sm text-ink no-underline hover:underline">
      <StarMark filled className="size-4 text-accent" />
      <span aria-hidden="true">
        <span className="font-semibold tabular-nums">{rating.average.toFixed(1)}</span> · {reviewsText(rating.count)}
      </span>
      <span className="sr-only">{`Rated ${starsLabel(rating.average)} from ${reviewsText(rating.count)}`}</span>
    </a>
  );
}

// RatingInline is the short rating of a browse row: a star, the average and the count.
export function RatingInline({ rating }: { rating: Rating }) {
  return (
    <span className="inline-flex items-center gap-1">
      <StarMark filled className="size-3 text-accent" />
      <span aria-hidden="true" className="tabular-nums">
        {rating.average.toFixed(1)} ({rating.count})
      </span>
      <span className="sr-only">{`rated ${starsLabel(rating.average)} from ${reviewsText(rating.count)}`}</span>
    </span>
  );
}
