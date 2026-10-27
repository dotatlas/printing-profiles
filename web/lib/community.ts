import { ApiError, call } from "./account";
import type { Comment, CommentPage, Preset, Review, ReviewPage } from "./api-types";

// checkID throws for anything that is not a public preset ID, so a bad value never reaches an API path.
function checkID(id: string): string {
  if (!/^[a-km-np-z2-9]{8}$/.test(id)) throw new Error("not a preset ID");
  return id;
}

// starsLabel says a rating in words, such as "4 stars out of 5". The star marks use it as their text.
export function starsLabel(stars: number): string {
  return `${stars} ${stars === 1 ? "star" : "stars"} out of 5`;
}

// pageQuery gives the query of a list call: the cursor of the next page, when there is one.
function pageQuery(cursor?: string): string {
  return cursor ? `?${new URLSearchParams({ cursor })}` : "";
}

// listReviews gets one page of a preset's reviews: the first page again after a change, or the next page for
// "Show more reviews". The reviews section calls it.
export async function listReviews(id: string, cursor?: string) {
  return call<ReviewPage>(`/api/v1/presets/${checkID(id)}/reviews${pageQuery(cursor)}`);
}

// getMyReview gives the signed-in member's review of a preset, or null when there is none. The form starts from it.
export async function getMyReview(id: string): Promise<Review | null> {
  try {
    return await call<Review>(`/api/v1/presets/${checkID(id)}/reviews/mine`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return null;
    throw err;
  }
}

// saveReview makes the member's review or replaces it, and deleteMyReview removes it. The review form calls them.
export async function saveReview(id: string, stars: number, body: string) {
  return call<Review>(`/api/v1/presets/${checkID(id)}/reviews`, "POST", { stars, body });
}

export async function deleteMyReview(id: string) {
  await call<void>(`/api/v1/presets/${checkID(id)}/reviews/mine`, "DELETE");
}

// listComments, addComment, editComment and deleteComment are the comment calls of the comments section.
export async function listComments(id: string, cursor?: string) {
  return call<CommentPage>(`/api/v1/presets/${checkID(id)}/comments${pageQuery(cursor)}`);
}

export async function addComment(id: string, body: string, parent?: string) {
  return call<Comment>(`/api/v1/presets/${checkID(id)}/comments`, "POST", parent ? { body, parent } : { body });
}

// checkCommentID throws for anything that is not a public comment ID.
function checkCommentID(id: string): string {
  if (!/^[a-km-np-z2-9]{12}$/.test(id)) throw new Error("not a comment ID");
  return id;
}

export async function editComment(id: string, body: string) {
  return call<Comment>(`/api/v1/comments/${checkCommentID(id)}`, "PATCH", { body });
}

export async function deleteComment(id: string) {
  await call<void>(`/api/v1/comments/${checkCommentID(id)}`, "DELETE");
}

// isBookmarked, addBookmark and removeBookmark are the calls of the bookmark button on the preset page.
export async function isBookmarked(id: string) {
  return (await call<{ bookmarked: boolean }>(`/api/v1/me/bookmarks/${checkID(id)}`)).bookmarked;
}

export async function addBookmark(id: string) {
  await call<void>("/api/v1/me/bookmarks", "POST", { preset: checkID(id) });
}

export async function removeBookmark(id: string) {
  await call<void>(`/api/v1/me/bookmarks/${checkID(id)}`, "DELETE");
}

// listBookmarks gets the member's bookmarked presets, newest bookmark first, for /my-bookmarks.
export async function listBookmarks() {
  return call<{ items: Preset[] }>("/api/v1/me/bookmarks");
}
