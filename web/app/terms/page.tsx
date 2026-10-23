import type { Metadata } from "next";

export const metadata: Metadata = { title: "Terms of use (draft)" };

// TermsPage holds the draft terms that onboarding asks people to accept, until the full terms replace them.
export default function TermsPage() {
  return (
    <article className="max-w-prose">
      <h1 className="text-2xl font-semibold tracking-tight">Terms of use (draft)</h1>
      <p className="mt-3 text-muted">
        These are draft terms. Full terms will replace them before the site opens to everyone, and you will be asked to
        accept those too.
      </p>
      <ol className="mt-6 list-decimal space-y-3 pl-5">
        <li>Share only presets and photos that you made or have the right to share.</li>
        <li>
          Presets you upload are shared under{" "}
          <a href="https://creativecommons.org/licenses/by/4.0/">CC BY 4.0</a> unless you choose another license for a
          preset.
        </li>
        <li>A result photo must show a real print made with the preset it belongs to.</li>
        <li>No spam, no harassment, and no content that is illegal where you live.</li>
        <li>Moderators can hide content that breaks these rules, and can suspend accounts that keep breaking them.</li>
      </ol>
    </article>
  );
}
