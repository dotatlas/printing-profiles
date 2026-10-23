"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { ApiError } from "../lib/account";
import type { Preset } from "../lib/api-types";
import { formatDate } from "../lib/format";
import { deletePreset, getMyPreset, listMyPresets, updatePreset } from "../lib/my-presets";
import { materialLabel, nozzleLabel, parsePresetParam, presetPath, printerLabel } from "../lib/presets";
import { Notice } from "./account-ui";
import { PresetForm } from "./preset-form";
import type { Catalog } from "./preset-list";
import { PresetView } from "./preset-view";
import { useMember, useSession } from "./session";

// Loading is the placeholder of a page that waits for the API. Screen readers hear "Loading".
function Loading() {
  return (
    <p role="status" className="mt-6 text-muted">
      Loading…
    </p>
  );
}

// savedPath gives the page to open after a save, with a mark that tells SavedNotice what happened: "published",
// "changes" (an edit of a published preset) or "draft". before is the status before the save.
function savedPath(p: Preset, before?: Preset["status"]): string {
  const mark = p.status === "draft" ? "draft" : before === "published" ? "changes" : "published";
  return `${presetPath(p)}?saved=${mark}`;
}

// useSavedMark reads the saved mark once and then takes it out of the address, so a reload does not repeat the
// message. SavedNotice and the draft view use it.
function useSavedMark(): string | null {
  const params = useSearchParams();
  const [mark] = useState(() => params.get("saved"));
  useEffect(() => {
    if (mark) history.replaceState(history.state, "", location.pathname);
  }, [mark]);
  return mark;
}

// SavedNotice confirms a save at the top of the preset page and takes the focus, so a screen reader reads it. After
// a publish it offers the link to copy, for the chat or forum thread where the author shares it.
export function SavedNotice() {
  const mark = useSavedMark();
  const box = useRef<HTMLDivElement>(null);
  const [copied, setCopied] = useState<"" | "done" | "failed">("");
  useEffect(() => {
    if (mark === "published" || mark === "changes") box.current?.focus();
  }, [mark]);
  if (mark !== "published" && mark !== "changes") return null;

  async function onCopy() {
    try {
      await navigator.clipboard.writeText(`${location.origin}${location.pathname}`);
      setCopied("done");
    } catch {
      setCopied("failed");
    }
  }

  return (
    <div ref={box} tabIndex={-1} className="focus-visible:outline-none">
      <Notice>
        {mark === "published" ? "Published. Anyone can find it now." : "Saved. The changes are on the public page."}
        {mark === "published" && (
          <>
            {" "}
            <button type="button" className="font-medium underline underline-offset-2" onClick={onCopy}>
              {copied === "done" ? "Link copied" : "Copy link"}
            </button>
            {copied === "failed" && " The browser did not allow it: copy the address from the address bar."}
          </>
        )}
      </Notice>
    </div>
  );
}

// NewPreset is the body of /presets/new: the empty form for a member. After a save it opens the preset's page.
export function NewPreset({ catalog }: { catalog: Catalog }) {
  const router = useRouter();
  const me = useMember("/presets/new");
  if (!me) return <Loading />;
  return <PresetForm catalog={catalog} onSaved={(p) => router.push(savedPath(p))} />;
}

type Loaded = { status: "loading" } | { status: "ready"; preset: Preset } | { status: "missing" } | { status: "failed" };

// usePresetForAuthor reads a preset with the session cookie once the member is known. It gives "missing" when the
// preset does not exist or belongs to someone else, because the API hides other people's drafts the same way.
function usePresetForAuthor(id: string, handle: string | null): Loaded {
  const [loaded, setLoaded] = useState<{ key: string; value: Loaded } | null>(null);
  const key = `${id}:${handle}`;
  useEffect(() => {
    if (!handle) return;
    let live = true;
    getMyPreset(id).then(
      (preset) => live && setLoaded({ key, value: preset.author.handle === handle ? { status: "ready", preset } : { status: "missing" } }),
      (err) =>
        live && setLoaded({ key, value: err instanceof ApiError && err.status === 404 ? { status: "missing" } : { status: "failed" } }),
    );
    return () => {
      live = false;
    };
  }, [id, handle, key]);
  return loaded?.key === key ? loaded.value : { status: "loading" };
}

// EditPreset is the body of /presets/<slug>-<id>/edit: the form with the preset, then the delete section. Only the
// author gets the form; anyone else reads that the preset is not theirs.
export function EditPreset({ catalog, id }: { catalog: Catalog; id: string }) {
  const pathname = usePathname();
  const me = useMember(pathname);
  const loaded = usePresetForAuthor(id, me?.handle ?? null);
  if (!me || loaded.status === "loading") return <Loading />;
  if (loaded.status === "missing") {
    return (
      <div className="mt-6">
        <Notice tone="error">You can edit only the presets that you shared, and this is not one of them.</Notice>
      </div>
    );
  }
  if (loaded.status === "failed") {
    return (
      <div className="mt-6">
        <Notice tone="error">The preset did not load. Reload the page to try again.</Notice>
      </div>
    );
  }
  const preset = loaded.preset;
  return (
    <>
      <p className="mt-2 text-muted wrap-anywhere">{preset.title}</p>
      <div className="mt-8">
        <PresetForm
          catalog={catalog}
          preset={preset}
          cancelHref={presetPath(preset)}
          // A full page load: Next keeps the preset page from the visit before, with its old data and state.
          onSaved={(p) => location.assign(savedPath(p, preset.status))}
        />
      </div>
      <DeleteSection id={preset.id} />
    </>
  );
}

// DeleteSection deletes the preset after a second step in place. The focus goes to Cancel first, so two presses of
// Enter cannot delete by accident.
function DeleteSection({ id }: { id: string }) {
  const router = useRouter();
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");

  async function onDelete() {
    setBusy(true);
    setProblem("");
    try {
      await deletePreset(id);
      router.push("/my-presets?deleted=1");
    } catch {
      setProblem("The preset was not deleted. Try again.");
      setBusy(false);
    }
  }

  return (
    <section aria-labelledby="delete" className="mt-12 max-w-2xl border-t border-line pt-8">
      <h2 id="delete" className="font-semibold">
        Delete this preset
      </h2>
      <p className="mt-1 text-sm text-muted">It goes away from the site and from your presets. You cannot undo this.</p>
      {problem && (
        <div className="mt-4">
          <Notice tone="error">{problem}</Notice>
        </div>
      )}
      {confirming ? (
        <div role="group" aria-label="Confirm the delete" className="mt-4 flex flex-wrap gap-3">
          <button type="button" className="btn btn-danger" disabled={busy} onClick={onDelete}>
            {busy ? "Deleting…" : "Delete for good"}
          </button>
          <button type="button" className="btn btn-secondary" disabled={busy} onClick={() => setConfirming(false)} autoFocus>
            Cancel
          </button>
        </div>
      ) : (
        <button type="button" className="btn btn-danger mt-4" onClick={() => setConfirming(true)}>
          Delete preset
        </button>
      )}
    </section>
  );
}

// AuthorActions shows the Edit button on a preset page when the signed-in member wrote the preset. The page is the
// same HTML for everyone, so this check runs in the browser.
export function AuthorActions({ preset }: { preset: Preset }) {
  const { state } = useSession();
  if (state.status !== "signed-in" || state.me.handle !== preset.author.handle) return null;
  return (
    <Link href={`${presetPath(preset)}/edit`} className="btn btn-secondary">
      Edit preset
    </Link>
  );
}

// PublishDraft publishes a draft from its own page in one step and opens the published page.
function PublishDraft({ preset }: { preset: Preset }) {
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  async function onPublish() {
    setBusy(true);
    setFailed(false);
    try {
      const published = await updatePreset(preset.id, { status: "published" });
      // A full page load: this is the not-found page of the same path, and a client navigation would keep it.
      location.assign(savedPath(published, "draft"));
    } catch {
      setFailed(true);
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex flex-wrap gap-3">
        <button type="button" className="btn btn-primary" disabled={busy} onClick={onPublish}>
          {busy ? "Publishing…" : "Publish"}
        </button>
        <Link href={`${presetPath(preset)}/edit`} className="btn btn-secondary">
          Edit preset
        </Link>
      </div>
      {failed && (
        <p role="alert" className="text-sm text-danger">
          The draft was not published. Try again.
        </p>
      )}
    </div>
  );
}

// MissingOrDraft is the not-found page of a preset path. A draft is a 404 for the server, which has no cookies, so
// the browser asks again with the session: the author sees the draft, anyone else sees "Page not found".
export function MissingOrDraft() {
  const pathname = usePathname();
  const { state } = useSession();
  const mark = useSavedMark();
  const id = parsePresetParam(pathname.split("/")[2] ?? "")?.id ?? "";
  const handle = state.status === "signed-in" ? state.me.handle || null : null;
  const loaded = usePresetForAuthor(id, id ? handle : null);
  const waiting = state.status === "loading" || (handle !== null && id !== "" && loaded.status === "loading");
  if (waiting) return <Loading />;
  if (loaded.status === "ready") {
    const preset = loaded.preset;
    const saved = mark === "draft" ? "Saved. " : "";
    return (
      <PresetView
        preset={preset}
        notice={
          <Notice>
            {preset.status === "draft"
              ? `${saved}This is a draft. Only you can see it. Publish it when it is ready.`
              : "Only you can see this preset."}
          </Notice>
        }
        actions={preset.status === "draft" ? <PublishDraft preset={preset} /> : undefined}
      />
    );
  }
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Page not found</h1>
      <p className="mt-2 text-muted">This preset does not exist, or its author deleted it.</p>
      <p className="mt-6">
        <Link href="/presets">Browse presets</Link>
      </p>
    </>
  );
}

// MyPresets is the body of /my-presets: the member's drafts, then the published presets, each group newest first.
export function MyPresets() {
  const me = useMember("/my-presets");
  const params = useSearchParams();
  // The delete message shows once; a reload of the page does not repeat it.
  const [deleted] = useState(() => params.get("deleted") === "1");
  useEffect(() => {
    if (deleted) history.replaceState(history.state, "", location.pathname);
  }, [deleted]);
  const [loaded, setLoaded] = useState<{ handle: string; items: Preset[] } | "failed" | null>(null);
  const handle = me?.handle ?? null;
  useEffect(() => {
    if (!handle) return;
    let live = true;
    listMyPresets().then(
      (page) => live && setLoaded({ handle, items: page.items }),
      () => live && setLoaded("failed"),
    );
    return () => {
      live = false;
    };
  }, [handle]);

  if (!me || loaded === null || (loaded !== "failed" && loaded.handle !== handle)) return <Loading />;
  if (loaded === "failed") {
    return (
      <div className="mt-6">
        <Notice tone="error">Your presets did not load. Reload the page to try again.</Notice>
      </div>
    );
  }
  return (
    <div className="mt-6 flex flex-col gap-6">
      {deleted && <Notice>The preset is deleted.</Notice>}
      {loaded.items.length === 0 ? (
        <div className="panel">
          <h2 className="font-semibold">You have not shared a preset yet</h2>
          <p className="mt-1 text-sm text-muted">Share the settings that work for your printer, so other people can use them.</p>
          <Link href="/presets/new" className="btn btn-primary mt-4">
            Share a preset
          </Link>
        </div>
      ) : (
        <>
          <PresetGroup title="Drafts" items={loaded.items.filter((p) => p.status === "draft")} />
          <PresetGroup title="Published" items={loaded.items.filter((p) => p.status !== "draft")} />
        </>
      )}
    </div>
  );
}

// PresetGroup is one group of My presets with its heading, or nothing when the group is empty.
function PresetGroup({ title, items }: { title: string; items: Preset[] }) {
  if (items.length === 0) return null;
  return (
    <section aria-label={title}>
      <h2 className="mb-2 text-sm font-medium">{title}</h2>
      <ul className="divide-y divide-line border-y border-line">
        {items.map((preset) => (
          <li key={preset.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
              <div className="min-w-0 flex-1 basis-64">
                <p className="flex flex-wrap items-baseline gap-x-2">
                  <Link href={presetPath(preset)} className="font-medium text-ink no-underline wrap-anywhere hover:underline">
                    {preset.title}
                  </Link>
                  {preset.status === "draft" && <span className="status-tag">Draft</span>}
                </p>
                <p className="mt-0.5 text-[13px] text-muted wrap-anywhere">
                  {printerLabel(preset.printer)} · {materialLabel(preset.material)} · {nozzleLabel(preset.nozzle_mm)} ·{" "}
                  {preset.slicer.name} · {preset.status === "draft" ? "Created" : "Published"}{" "}
                  {formatDate(preset.published_at ?? preset.created_at)}
                </p>
              </div>
              <Link href={`${presetPath(preset)}/edit`} className="btn btn-secondary">
                {/* The space stays outside the hidden span: a name computed from the span alone drops it. */}
                Edit <span className="sr-only">{preset.title}</span>
              </Link>
            </li>
          ))}
      </ul>
    </section>
  );
}
