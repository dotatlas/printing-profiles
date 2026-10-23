"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import { ApiError, finishOnboarding, safeReturn } from "../lib/account";
import { Notice } from "./account-ui";
import { useSession } from "./session";

const handleRule = /^[a-z0-9]+(-[a-z0-9]+)*$/;

// toHandle makes typed text into handle form as the visitor types: lower case, letters without accents ("ś" becomes
// "s"), and dashes for spaces and underscores.
export function toHandle(text: string): string {
  return text.normalize("NFKD").replace(/\p{M}/gu, "").toLowerCase().replace(/[\s_]+/g, "-").replace(/[^a-z0-9-]/g, "").replace(/-{2,}/g, "-").slice(0, 30);
}

// handleProblem says what is wrong with a handle, or gives "" when the handle has the right form. Welcome uses it.
export function handleProblem(handle: string): string {
  if (handle.length < 3) return "A handle has at least 3 characters.";
  if (!handleRule.test(handle)) return "Start and end with a letter or a digit, with single dashes between.";
  return "";
}

// Welcome is the first-sign-in step: the visitor picks a public handle and accepts the terms, then goes on to the
// return path. A visitor who already has a handle goes on at once. The /welcome route renders it.
export function Welcome() {
  const params = useSearchParams();
  const router = useRouter();
  const { state, refresh } = useSession();
  const returnPath = safeReturn(params.get("return"));
  const [handle, setHandle] = useState("");
  const [accepted, setAccepted] = useState(false);
  const [touched, setTouched] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (state.status === "signed-out") router.replace(`/signin?${new URLSearchParams({ return: returnPath })}`);
    if (state.status === "signed-in" && !state.me.needs_onboarding) router.replace(returnPath);
  }, [state, returnPath, router]);

  const problem = handleProblem(handle);
  const showProblem = touched && handle !== "" && problem !== "";

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setTouched(true);
    setSubmitted(true);
    if (problem || !accepted) return;
    setBusy(true);
    setError("");
    try {
      await finishOnboarding(handle);
      await refresh();
      router.replace(returnPath);
    } catch (err) {
      setBusy(false);
      if (err instanceof ApiError && err.status === 409) setError("That handle is taken. Try another one.");
      else if (err instanceof ApiError && err.status === 0) setError(err.detail);
      else setError("Your handle could not be saved. Try again.");
    }
  }

  if (state.status !== "signed-in" || !state.me.needs_onboarding) {
    return (
      <p role="status" className="mt-2 text-muted">
        Loading…
      </p>
    );
  }

  return (
    <>
      <p className="mt-2 text-muted">
        Your handle is your public name on presets and reviews. It cannot be changed later, so pick one you will keep.
      </p>
      <form onSubmit={onSubmit} noValidate className="mt-8">
        <label htmlFor="handle" className="block text-sm font-medium">
          Handle
        </label>
        <div className="mt-2 flex items-center rounded-[0.625rem] focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-accent">
          <span className="flex h-10 items-center rounded-l-[0.625rem] border border-r-0 border-control bg-raised px-3 text-muted" aria-hidden="true">
            @
          </span>
          <input
            id="handle"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            required
            value={handle}
            onChange={(e) => {
              setHandle(toHandle(e.target.value));
              setError("");
            }}
            onBlur={() => setTouched(true)}
            aria-invalid={showProblem || error !== ""}
            aria-describedby="handle-help"
            className="field rounded-l-none focus-visible:outline-none"
          />
        </div>
        <p id="handle-help" className={`mt-2 text-sm ${showProblem || error ? "text-danger" : "text-muted"}`}>
          {error || (showProblem ? problem : "3 to 30 characters: lowercase letters, digits and dashes.")}
        </p>

        <label className="mt-6 flex items-start gap-3 text-sm">
          <input
            type="checkbox"
            checked={accepted}
            onChange={(e) => setAccepted(e.target.checked)}
            className="mt-0.5 size-4 shrink-0 accent-accent"
          />
          <span>
            I accept the{" "}
            {/* A new tab, so the handle typed above stays here. The label starts with the visible words. */}
            <Link href="/terms" target="_blank" rel="noopener" aria-label="terms of use (opens in a new tab)">
              terms of use
            </Link>
            .
          </span>
        </label>
        {submitted && !accepted && (
          <div className="mt-4">
            <Notice tone="error">Accept the terms to continue.</Notice>
          </div>
        )}

        <button type="submit" className="btn btn-primary mt-6 w-full" disabled={busy}>
          {busy ? "Saving…" : "Continue"}
        </button>
      </form>
    </>
  );
}
