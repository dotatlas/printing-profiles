import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Questions and answers",
  description: "What \"Photo verified\" means on Printing Profiles, what it does not prove, and how a preset gets it.",
};

// FaqPage answers the questions about the site's trust marks. The "Photo verified" tag on a preset page links to its
// first answer.
export default function FaqPage() {
  return (
    <article className="max-w-prose">
      <h1 className="text-2xl font-semibold tracking-tight">Questions and answers</h1>

      <section aria-labelledby="photo-verified" className="mt-8 scroll-mt-6">
        <h2 id="photo-verified" className="text-lg font-semibold">
          What does &ldquo;Photo verified&rdquo; mean?
        </h2>
        <p className="mt-2">
          The author added at least one photo of a print that they made with the preset. The photos are on the preset page,
          above its settings.
        </p>
      </section>

      <section aria-labelledby="not-proof" className="mt-8">
        <h2 id="not-proof" className="text-lg font-semibold">
          What does it not prove?
        </h2>
        <ul className="mt-2 list-disc space-y-2 pl-5">
          <li>Nobody at Printing Profiles checks that the photo came from this preset. A photo cannot prove which settings made a print.</li>
          <li>It does not promise the same result on your machine: printers, filament batches and rooms differ.</li>
          <li>
            Reviews from people who tried the preset say more. Read them, and add yours when you print it.
          </li>
        </ul>
      </section>

      <section aria-labelledby="get-it" className="mt-8">
        <h2 id="get-it" className="text-lg font-semibold">
          How does a preset get it?
        </h2>
        <p className="mt-2">
          Open the preset&rsquo;s edit page and add a photo of a print in the Photos section. The site makes a new copy of
          every photo, so the location and the camera data in the original file are not kept.
        </p>
      </section>

      <section aria-labelledby="wrong-photo" className="mt-8">
        <h2 id="wrong-photo" className="text-lg font-semibold">
          A photo looks wrong. What can I do?
        </h2>
        <p className="mt-2">
          Say so in the comments of the preset, so the author and other visitors see it. Photos must show a real print made
          with the preset (see the <Link href="/terms">terms</Link>).
        </p>
      </section>
    </article>
  );
}
