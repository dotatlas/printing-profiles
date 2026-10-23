"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { type FormEvent, type ReactNode, useCallback, useEffect, useState } from "react";
import {
  ApiError,
  endSession,
  listIdentities,
  listSessions,
  oauthStartURL,
  type Provider,
  removeIdentity,
  signOutEverywhere,
  signOutOthers,
  updateDisplayName,
} from "../lib/account";
import type { Identity, Me, Session } from "../lib/api-types";
import { describeDevice, formatDate, timeAgo } from "../lib/format";
import { Notice, ProviderMark, providerName } from "./account-ui";
import { useMember, useSession } from "./session";

// Section is one settings area: its title and description on the left, its panel on the right (stacked on a phone).
function Section({ id, title, description, children }: { id: string; title: string; description: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="grid gap-4 border-t border-line py-8 first:border-t-0 first:pt-0 md:grid-cols-[13rem_1fr] md:gap-8">
      <div className="md:pt-3">
        {/* tabIndex -1 lets a panel move the focus here after the row with the focus goes away. */}
        <h2 id={id} tabIndex={-1} className="font-semibold focus-visible:outline-none">
          {title}
        </h2>
        <p className="mt-1 text-sm text-muted">{description}</p>
      </div>
      <div className="panel min-w-0">{children}</div>
    </section>
  );
}

// Skeleton holds a panel's place while its data loads: bars at the size of the rows that will come.
function Skeleton({ rows }: { rows: number }) {
  return (
    <div role="status" className="flex flex-col gap-3">
      <span className="sr-only">Loading</span>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="h-10 rounded-[0.625rem] bg-line/60" />
      ))}
    </div>
  );
}

// LoadError says that a panel's data did not load, with a way to try again.
function LoadError({ what, onRetry }: { what: string; onRetry: () => void }) {
  return (
    <p className="text-sm text-muted">
      Could not load your {what}.{" "}
      <button type="button" onClick={onRetry} className="text-accent underline underline-offset-2">
        Try again
      </button>
    </p>
  );
}

// useLoad loads a list for a panel and gives its state and a reload. The sign-in methods and sessions panels use it.
function useLoad<T>(load: () => Promise<T[]>) {
  const [items, setItems] = useState<T[] | null>(null);
  const [failed, setFailed] = useState(false);
  const reload = useCallback(async () => {
    setFailed(false);
    try {
      setItems(await load());
    } catch {
      setFailed(true);
    }
  }, [load]);
  useEffect(() => {
    let live = true;
    load().then(
      (rows) => live && setItems(rows),
      () => live && setFailed(true),
    );
    return () => {
      live = false;
    };
  }, [load]);
  return { items, failed, reload };
}

// ProfilePanel shows the fixed handle and edits the display name, with one Save for the panel.
function ProfilePanel({ me, onSaved }: { me: Me; onSaved: () => Promise<void> }) {
  const [name, setName] = useState(me.display_name);
  const [status, setStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const changed = name.trim() !== me.display_name;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setStatus("saving");
    try {
      await updateDisplayName(name.trim());
      await onSaved();
      setStatus("saved");
    } catch {
      setStatus("error");
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate>
      <p className="text-sm font-medium">Handle</p>
      <p className="mt-1 wrap-anywhere">@{me.handle}</p>
      <p className="mt-1 text-sm text-muted">A handle cannot be changed.</p>

      <label htmlFor="display-name" className="mt-6 block text-sm font-medium">
        Display name <span className="font-normal text-muted">(optional)</span>
      </label>
      <input
        id="display-name"
        maxLength={60}
        autoComplete="name"
        value={name}
        onChange={(e) => {
          setName(e.target.value);
          setStatus("idle");
        }}
        className="field mt-2"
      />
      <p className="mt-2 text-sm text-muted">Shown beside your handle. Up to 60 characters.</p>
      <div className="mt-4 flex items-center justify-end gap-3">
        {status === "saved" && (
          <span role="status" className="saved-stamp text-sm text-muted">
            Saved
          </span>
        )}
        {status === "error" && (
          <span role="alert" className="text-sm text-danger">
            Not saved. Try again.
          </span>
        )}
        <button type="submit" className="btn btn-primary" disabled={!changed || status === "saving"}>
          {status === "saving" ? "Saving…" : "Save"}
        </button>
      </div>
    </form>
  );
}

const linkErrors: Record<string, string> = {
  "linked-elsewhere": "That account already belongs to another Printing Profiles account, so it was not added.",
};

// MethodsPanel lists the sign-in methods, removes one (never the last) and adds GitHub or Discord.
function MethodsPanel({ error }: { error: string }) {
  const { items, failed, reload } = useLoad<Identity>(listIdentities);
  const [removing, setRemoving] = useState<number | null>(null);
  const [problem, setProblem] = useState(linkErrors[error] ?? "");

  async function onRemove(id: number) {
    setRemoving(id);
    setProblem("");
    try {
      await removeIdentity(id);
      await reload();
      document.getElementById("methods")?.focus(); // The removed row had the focus.
    } catch (err) {
      setProblem(
        err instanceof ApiError && err.status === 409
          ? "That is your only way to sign in, so it stays. Add another one first."
          : "The sign-in method was not removed. Try again.",
      );
    } finally {
      setRemoving(null);
    }
  }

  if (failed) return <LoadError what="sign-in methods" onRetry={reload} />;
  if (!items) return <Skeleton rows={2} />;
  const only = items.length === 1;
  const missing = (["github", "discord"] as Provider[]).filter((p) => !items.some((i) => i.provider === p));

  return (
    <>
      {problem && (
        <div className="mb-4">
          <Notice tone="error">{problem}</Notice>
        </div>
      )}
      <ul className="divide-y divide-line">
        {items.map((m) => (
          <li key={m.id} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0">
            <ProviderMark provider={m.provider} />
            <div className="min-w-0 flex-1">
              <p className="font-medium">{providerName(m.provider)}</p>
              {m.email && (
                <p className="truncate text-sm text-muted" title={m.email}>
                  {m.email}
                </p>
              )}
              <p className="text-sm text-muted">
                Added {formatDate(m.created_at)}
                {only && " · your only way to sign in"}
              </p>
            </div>
            {!only && (
              <button type="button" className="btn btn-secondary" disabled={removing !== null} onClick={() => onRemove(m.id)}>
                {/* The space stays outside the hidden span: a name computed from the span alone drops it. */}
                {removing === m.id ? "Removing…" : "Remove"} <span className="sr-only">{providerName(m.provider)}</span>
              </button>
            )}
          </li>
        ))}
      </ul>
      {missing.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-3 border-t border-line pt-4">
          {missing.map((p) => (
            <a key={p} href={oauthStartURL(p, "/settings", true)} className="btn btn-secondary">
              <ProviderMark provider={p} />
              Add {providerName(p)}
            </a>
          ))}
        </div>
      )}
    </>
  );
}

// SessionsPanel lists where the user is signed in. It signs out one device, every other device, or every device
// after a confirmation.
function SessionsPanel({ onSignedOutEverywhere }: { onSignedOutEverywhere: () => Promise<void> }) {
  const { items, failed, reload } = useLoad<Session>(listSessions);
  const [ending, setEnding] = useState<number | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");
  const [note, setNote] = useState("");

  async function onEnd(id: number) {
    setEnding(id);
    setProblem("");
    try {
      await endSession(id);
      await reload();
      document.getElementById("sessions")?.focus(); // The signed-out row had the focus.
    } catch {
      setProblem("That device was not signed out. Try again.");
    } finally {
      setEnding(null);
    }
  }

  async function onOthers() {
    setBusy(true);
    setProblem("");
    try {
      const { signed_out: n } = await signOutOthers();
      await reload();
      setNote(n === 1 ? "1 other device is signed out." : `${n} other devices are signed out.`);
      document.getElementById("sessions")?.focus();
    } catch {
      setProblem("The other devices were not signed out. Try again.");
    } finally {
      setBusy(false);
    }
  }

  async function onEverywhere() {
    setBusy(true);
    try {
      await signOutEverywhere();
      await onSignedOutEverywhere();
    } catch {
      setBusy(false);
      setProblem("You were not signed out. Try again.");
    }
  }

  if (failed) return <LoadError what="sessions" onRetry={reload} />;
  if (!items) return <Skeleton rows={2} />;

  return (
    <>
      {problem && (
        <div className="mb-4">
          <Notice tone="error">{problem}</Notice>
        </div>
      )}
      {note && (
        <div className="mb-4">
          <Notice>{note}</Notice>
        </div>
      )}
      <ul className="flex flex-col gap-1">
        {items.map((s) => (
          <li
            key={s.id}
            className={`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[0.625rem] px-3 py-2.5 ${s.current ? "bg-accent-tint" : ""}`}
          >
            <div className="min-w-0 flex-1">
              <p className="flex flex-wrap items-center gap-x-2 font-medium">
                {describeDevice(s.user_agent)}
                {s.current && <span className="text-sm font-normal text-accent-ink">This device</span>}
              </p>
              <p className="text-sm text-muted">
                Active {timeAgo(s.last_seen_at)} · Signed in on {formatDate(s.created_at)}
              </p>
            </div>
            {!s.current && (
              <button type="button" className="btn btn-secondary" disabled={ending !== null} onClick={() => onEnd(s.id)}>
                {ending === s.id ? "Signing out…" : "Sign out"}{" "}
                <span className="sr-only">
                  {describeDevice(s.user_agent)}, active {timeAgo(s.last_seen_at)}
                </span>
              </button>
            )}
          </li>
        ))}
      </ul>
      <div className="mt-4 border-t border-line pt-4">
        {confirming ? (
          <div role="group" aria-label="Confirm signing out everywhere">
            <p className="text-sm">
              This signs you out on {items.length === 1 ? "this device" : `all ${items.length} devices, including this one`}.
            </p>
            {/* The focus starts on Cancel, so pressing Enter twice cannot sign out everywhere by accident. */}
            <div className="mt-3 flex flex-wrap gap-3">
              <button type="button" className="btn btn-danger" disabled={busy} onClick={onEverywhere}>
                {busy ? "Signing out…" : "Sign out everywhere"}
              </button>
              <button type="button" className="btn btn-secondary" disabled={busy} onClick={() => setConfirming(false)} autoFocus>
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div className="flex flex-wrap gap-3">
            {items.length > 1 && (
              <button type="button" className="btn btn-secondary" disabled={busy} onClick={onOthers}>
                {busy ? "Signing out…" : "Sign out other devices"}
              </button>
            )}
            <button type="button" className="btn btn-danger" disabled={busy} onClick={() => setConfirming(true)}>
              Sign out everywhere
            </button>
          </div>
        )}
      </div>
    </>
  );
}

// Settings is the account settings page: profile, sign-in methods and sessions. A signed-out visitor goes to sign-in,
// and a new user to onboarding first. The /settings route renders it.
export function Settings() {
  const router = useRouter();
  const params = useSearchParams();
  const { refresh } = useSession();
  const me = useMember("/settings");

  return (
    <>
      <div className="mt-8">
        <Section id="profile" title="Profile" description="Your public handle and the name shown next to it.">
          {me ? <ProfilePanel key={me.display_name} me={me} onSaved={refresh} /> : <Skeleton rows={3} />}
        </Section>
        <Section id="methods" title="Sign-in methods" description="The ways you can sign in. Keep at least one.">
          {me ? <MethodsPanel error={params.get("error") ?? ""} /> : <Skeleton rows={2} />}
        </Section>
        <Section
          id="sessions"
          title="Sessions"
          description="Where you are signed in. Sign out of any device you do not recognize."
        >
          {me ? (
            <SessionsPanel
              onSignedOutEverywhere={async () => {
                await refresh();
                router.replace("/");
              }}
            />
          ) : (
            <Skeleton rows={2} />
          )}
        </Section>
      </div>
    </>
  );
}
