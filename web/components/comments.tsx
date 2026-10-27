"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { type FormEvent, useId, useState } from "react";
import { ApiError, waitText, welcomePath } from "../lib/account";
import type { Comment } from "../lib/api-types";
import { addComment, deleteComment, editComment, listComments } from "../lib/community";
import { formatDate } from "../lib/format";
import { Notice } from "./account-ui";
import { useSession } from "./session";

const maxComment = 2000;

// problemOf says why a comment write failed, for the line under its form.
function problemOf(err: unknown): string {
  if (err instanceof ApiError && err.status === 429) return `You wrote many comments in the last hour. Try again ${waitText(err.retryAfter)}.`;
  return "The comment was not saved. Try again.";
}

// CommentForm is one text box with its button: a new comment, a reply, or an edit. onSend gets the trimmed text
// and throws when the write fails; onCancel, when given, shows a Cancel button.
function CommentForm({
  label,
  button,
  start = "",
  onSend,
  onCancel,
}: {
  label: string;
  button: string;
  start?: string;
  onSend: (body: string) => Promise<void>;
  onCancel?: () => void;
}) {
  const id = useId();
  const [body, setBody] = useState(start);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState("");

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (body.trim() === "") {
      setProblem("Write something first.");
      return;
    }
    setBusy(true);
    setProblem("");
    try {
      await onSend(body.trim());
      setBody("");
    } catch (err) {
      setProblem(problemOf(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate className="flex max-w-2xl flex-col gap-2">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <textarea
        id={id}
        className="field h-auto min-h-20 py-2"
        value={body}
        onChange={(e) => setBody(e.target.value)}
        maxLength={maxComment}
        aria-invalid={problem ? true : undefined}
        // A reply or an edit opens on a press, so the text box takes the focus at once.
        autoFocus={start !== "" || onCancel !== undefined}
      />
      {problem && <p className="text-sm text-danger">{problem}</p>}
      <div className="flex flex-wrap gap-3">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {button}
        </button>
        {onCancel && (
          <button type="button" className="btn btn-secondary" onClick={onCancel}>
            Cancel
          </button>
        )}
      </div>
    </form>
  );
}

// CommentItem is one comment with its own actions: Reply under a top-level comment, Edit and Delete for its author.
// Replies sit under it, indented.
function CommentItem({
  comment,
  presetId,
  meHandle,
  top,
  onChange,
}: {
  comment: Comment;
  presetId: string;
  meHandle: string | null;
  top: boolean;
  onChange: (note?: string) => Promise<void>;
}) {
  const [mode, setMode] = useState<"read" | "reply" | "edit" | "confirm">("read");
  const [problem, setProblem] = useState("");
  const mine = meHandle !== null && comment.author.handle === meHandle && !comment.deleted;
  const name = comment.author.display_name || `@${comment.author.handle}`;

  async function onDelete() {
    setProblem("");
    try {
      await deleteComment(comment.id);
      setMode("read");
      // The pressed button can leave with its comment, so the focus goes to the section heading.
      document.getElementById("comments")?.focus();
      await onChange("Your comment is deleted.");
    } catch {
      setProblem("The comment was not deleted. Try again.");
    }
  }

  return (
    <article className="flex flex-col gap-2">
      {comment.deleted ? (
        <p className="text-sm text-muted">Comment deleted</p>
      ) : (
        <>
          <p className="flex flex-wrap items-baseline gap-x-3 text-sm">
            <span className="font-medium wrap-anywhere">{name}</span>
            <span className="text-muted">
              {formatDate(comment.created_at)}
              {comment.edited && " · edited"}
            </span>
          </p>
          {mode === "edit" ? (
            <CommentForm
              label="Edit your comment"
              button="Save"
              start={comment.body}
              onSend={async (body) => {
                await editComment(comment.id, body);
                setMode("read");
                await onChange();
              }}
              onCancel={() => setMode("read")}
            />
          ) : (
            <p className="max-w-prose whitespace-pre-line wrap-anywhere">{comment.body}</p>
          )}
        </>
      )}
      {mode === "confirm" ? (
        <div role="group" aria-label="Confirm the delete" className="flex flex-col gap-2">
          <p className="text-sm">Delete this comment?</p>
          <div className="flex flex-wrap gap-3">
            <button type="button" className="btn btn-danger" onClick={onDelete}>
              Delete for good
            </button>
            <button type="button" className="btn btn-secondary" onClick={() => setMode("read")} autoFocus>
              Cancel
            </button>
          </div>
        </div>
      ) : (
        mode === "read" &&
        meHandle !== null && (
          <div className="flex flex-wrap gap-4 text-sm">
            {/* The space stays outside the hidden span: a name computed from the span alone drops it. */}
            {top && !comment.deleted && (
              <button type="button" className="text-accent underline underline-offset-2" onClick={() => setMode("reply")}>
                Reply <span className="sr-only">to {name}</span>
              </button>
            )}
            {mine && (
              <>
                <button type="button" className="text-accent underline underline-offset-2" onClick={() => setMode("edit")}>
                  Edit <span className="sr-only">your comment</span>
                </button>
                <button type="button" className="text-accent underline underline-offset-2" onClick={() => setMode("confirm")}>
                  Delete <span className="sr-only">your comment</span>
                </button>
              </>
            )}
          </div>
        )
      )}
      {problem && <Notice tone="error">{problem}</Notice>}
      {comment.replies && comment.replies.length > 0 && (
        <ul className="mt-2 flex flex-col gap-5 border-l border-line pl-4">
          {comment.replies.map((r) => (
            <li key={r.id}>
              <CommentItem comment={r} presetId={presetId} meHandle={meHandle} top={false} onChange={onChange} />
            </li>
          ))}
        </ul>
      )}
      {/* The form comes after the replies, where the new reply shows. */}
      {mode === "reply" && (
        <div className="mt-1 border-l border-line pl-4">
          <CommentForm
            label="Your reply"
            button="Post reply"
            onSend={async (body) => {
              await addComment(presetId, body, comment.id);
              setMode("read");
              await onChange();
            }}
            onCancel={() => setMode("read")}
          />
        </div>
      )}
    </article>
  );
}

// Comments is the comments section of the preset page: the thread oldest first, each top-level comment with its
// replies, and the form for a new comment. The server gives the first page; the section reads it again after changes.
export function Comments({ presetId, initial }: { presetId: string; initial: Comment[] }) {
  const { state } = useSession();
  const pathname = usePathname();
  const [items, setItems] = useState(initial);
  const [note, setNote] = useState("");
  const me = state.status === "signed-in" && !state.me.needs_onboarding ? state.me : null;

  // reload reads the thread again and puts the note, if any, on the status line.
  async function reload(next = "") {
    setNote(next);
    try {
      setItems((await listComments(presetId)).items);
    } catch {
      // The thread stays as it was; the next page load shows the change.
    }
  }

  return (
    <section aria-labelledby="comments" className="scroll-mt-6">
      <h2 id="comments" tabIndex={-1} className="font-semibold focus:outline-none">
        Comments
      </h2>
      {/* The status line stays in the page, so a screen reader reads each new note. */}
      <p role="status" className="mt-2 text-sm text-muted empty:hidden">
        {note}
      </p>
      {items.length === 0 ? (
        <p className="mt-3 text-muted">No comments yet. Ask the author about the setup, or say what you changed.</p>
      ) : (
        <ul className="mt-4 flex flex-col gap-6">
          {items.map((t) => (
            <li key={t.id}>
              <CommentItem comment={t} presetId={presetId} meHandle={me?.handle ?? null} top onChange={reload} />
            </li>
          ))}
        </ul>
      )}
      <div className="mt-6">
        {state.status === "signed-out" && (
          <Link href={`/signin?return=${encodeURIComponent(`${pathname}#comments`)}`}>Sign in to comment</Link>
        )}
        {state.status === "signed-in" && state.me.needs_onboarding && (
          <Link href={welcomePath(`${pathname}#comments`)}>Choose your handle to comment</Link>
        )}
        {me && (
          <CommentForm
            label="Add a comment"
            button="Post comment"
            onSend={async (body) => {
              await addComment(presetId, body);
              await reload("Your comment is posted.");
            }}
          />
        )}
      </div>
    </section>
  );
}
