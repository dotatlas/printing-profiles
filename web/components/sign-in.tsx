"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { type FormEvent, useEffect, useRef, useState } from "react";
import { ApiError, oauthStartURL, type Provider, safeReturn, startEmailSignIn, waitText, welcomePath } from "../lib/account";
import { Notice, ProviderMark, providerName } from "./account-ui";
import { useSession } from "./session";

// redirectErrors are the messages for the error codes that the API's OAuth callback puts on /signin.
const redirectErrors: Record<string, string> = {
  cancelled: "Sign-in was cancelled. Nothing changed, so you can try again.",
  provider: "GitHub or Discord did not answer. Try again in a moment, or use your email address.",
  "link-session": "To add a sign-in method, open Settings while you are signed in and add it there.",
};

// emailError turns a failed email request into words that say what to do next. SignIn uses it.
function emailError(err: unknown): string {
  if (!(err instanceof ApiError)) return "Something went wrong. Try again.";
  if (err.status === 0) return err.detail;
  if (err.status === 400) return "Enter one email address, like name@example.com.";
  if (err.status === 429) return `Too many attempts. Try again ${waitText(err.retryAfter)}.`;
  if (err.status === 503) return "Email sign-in is not available right now. Use GitHub or Discord.";
  return "The sign-in link could not be sent. Try again in a few minutes.";
}

type EmailState = { step: "form" } | { step: "sending" } | { step: "sent"; email: string } | { step: "error"; message: string };

// SignIn is the sign-in page: GitHub and Discord buttons, then an email field that sends a one-time link.
// The /signin route renders it. A signed-in visitor goes straight on to the return path.
export function SignIn() {
  const params = useSearchParams();
  const router = useRouter();
  const { state } = useSession();
  const returnPath = safeReturn(params.get("return"));
  const afterSignIn = welcomePath(returnPath);
  const redirectError = redirectErrors[params.get("error") ?? ""];
  const [email, setEmail] = useState("");
  const [emailState, setEmailState] = useState<EmailState>({ step: "form" });
  const [resent, setResent] = useState(false);
  const sentHeading = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    if (state.status === "signed-in") router.replace(returnPath);
  }, [state.status, returnPath, router]);

  // The submit button goes away with the form, so the focus moves to the new heading, which screen readers then read.
  useEffect(() => {
    if (emailState.step === "sent") sentHeading.current?.focus();
  }, [emailState.step]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (email.trim() === "") {
      setEmailState({ step: "error", message: "Enter your email address." });
      return;
    }
    setEmailState({ step: "sending" });
    try {
      await startEmailSignIn(email, afterSignIn);
      setResent(false);
      setEmailState({ step: "sent", email: email.trim() });
    } catch (err) {
      setEmailState({ step: "error", message: emailError(err) });
    }
  }

  async function onResend(address: string) {
    try {
      await startEmailSignIn(address, afterSignIn);
      setResent(true);
    } catch (err) {
      setEmailState({ step: "error", message: emailError(err) });
    }
  }

  const providers: Provider[] = ["github", "discord"];
  return (
    <>
      <p className="mt-2 text-muted">
        Browse and download presets without an account. Sign in to upload, review and save them.
      </p>

      {redirectError && (
        <div className="mt-6">
          <Notice tone="error">{redirectError}</Notice>
        </div>
      )}

      <div className="mt-8 flex flex-col gap-3">
        {providers.map((p) => (
          <a key={p} href={oauthStartURL(p, afterSignIn)} className="btn btn-secondary w-full">
            <ProviderMark provider={p} />
            Continue with {providerName(p)}
          </a>
        ))}
      </div>

      <p className="my-8 flex items-center gap-3 text-sm text-muted before:h-px before:flex-1 before:bg-line after:h-px after:flex-1 after:bg-line">
        or
      </p>

      {emailState.step === "sent" ? (
        <div className="panel">
          <h2 ref={sentHeading} tabIndex={-1} className="font-semibold focus-visible:outline-none">
            Check your inbox
          </h2>
          <p className="mt-1 text-sm text-muted">
            We sent a sign-in link to <span className="font-medium wrap-anywhere text-ink">{emailState.email}</span>. It works
            once, for 15 minutes.
          </p>
          <p className="mt-3 text-sm text-muted">
            Not there? Check your spam folder, or{" "}
            <button type="button" className="text-accent underline underline-offset-2" onClick={() => onResend(emailState.email)}>
              send it again
            </button>
            .
          </p>
          {resent && (
            <p role="status" className="mt-2 text-sm text-muted">
              Sent again. The newest link is the one to use.
            </p>
          )}
          <p className="mt-3 text-sm">
            <button type="button" className="text-accent underline underline-offset-2" onClick={() => setEmailState({ step: "form" })}>
              Use a different address
            </button>
          </p>
        </div>
      ) : (
        <form onSubmit={onSubmit} noValidate>
          <label htmlFor="email" className="block text-sm font-medium">
            Email address
          </label>
          <input
            id="email"
            type="email"
            inputMode="email"
            autoComplete="email"
            required
            maxLength={254}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            aria-invalid={emailState.step === "error"}
            aria-describedby={emailState.step === "error" ? "email-error" : undefined}
            className="field mt-2"
          />
          {emailState.step === "error" && (
            <p id="email-error" className="mt-2 text-sm text-danger">
              {emailState.message}
            </p>
          )}
          <button type="submit" className="btn btn-primary mt-4 w-full" disabled={emailState.step === "sending"}>
            {emailState.step === "sending" ? "Sending…" : "Email me a sign-in link"}
          </button>
        </form>
      )}

      <p className="mt-8 text-sm text-muted">There are no passwords here, and your email address is never shown to other people.</p>
    </>
  );
}
