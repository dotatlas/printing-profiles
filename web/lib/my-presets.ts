import { ApiError, call } from "./account";
import type { Photo, Preset, PresetCreate, PresetUpdate, Upload, UploadTicket } from "./api-types";

// formFields are the fields of PresetCreate that an API message can name. The longest name comes first, so
// "slicer_version" wins over "slicer".
const formFields = ["slicer_version", "description", "nozzle_mm", "material", "printer", "license", "slicer", "title", "file"] as const;

export type FormField = (typeof formFields)[number];

// fieldOfProblem gives the form field that a 400 message from the API is about, or null. The API starts each
// message with the field name, so the preset form can show the message under the field.
export function fieldOfProblem(detail: string): FormField | null {
  return formFields.find((field) => detail.startsWith(`${field} `)) ?? null;
}

// checkID throws for anything that is not a public preset ID, so a bad value never reaches an API path.
function checkID(id: string): string {
  if (!/^[a-km-np-z2-9]{8}$/.test(id)) throw new Error("not a preset ID");
  return id;
}

// getMyPreset reads one preset with the session cookie, so the author also gets a draft. The preset form and the
// draft view on the preset page call it.
export function getMyPreset(id: string) {
  return call<Preset>(`/api/v1/presets/${checkID(id)}`);
}

// listMyPresets gives the signed-in user's presets, drafts included. The My presets page calls it.
export function listMyPresets() {
  return call<{ items: Preset[] }>("/api/v1/me/presets");
}

// createPreset, updatePreset and deletePreset are the writes of the preset form. The API checks every field.
export function createPreset(input: PresetCreate) {
  return call<Preset>("/api/v1/presets", "POST", input);
}

export function updatePreset(id: string, input: PresetUpdate) {
  return call<Preset>(`/api/v1/presets/${checkID(id)}`, "PATCH", input);
}

export async function deletePreset(id: string) {
  await call<void>(`/api/v1/presets/${checkID(id)}`, "DELETE");
}

// uploadPresetFile sends a preset file and gives what the check found. The preset form calls it.
export function uploadPresetFile(file: File): Promise<Upload> {
  return upload(file, "preset_file");
}

// addPhoto sends a photo, has it checked, and adds it to the member's own preset with its alt text. The photos
// section of the edit page calls it. removePhoto takes a photo off the preset.
export async function addPhoto(presetId: string, file: File, alt: string): Promise<Photo> {
  const checked = await upload(file, "photo");
  return call<Photo>(`/api/v1/presets/${checkID(presetId)}/photos`, "POST", { upload: checked.id, alt });
}

export async function removePhoto(id: string) {
  if (!/^[a-km-np-z2-9]{12}$/.test(id)) throw new Error("not a photo ID");
  await call<void>(`/api/v1/photos/${id}`, "DELETE");
}

// upload sends a file in three steps: a ticket from the API, the file straight to the bucket URL of the ticket, then
// "complete", which checks it. uploadPresetFile and addPhoto use it.
async function upload(file: File, kind: "preset_file" | "photo"): Promise<Upload> {
  const ticket = await call<UploadTicket>("/api/v1/uploads", "POST", { kind, size: file.size });
  let sent: Response;
  try {
    // No credentials: the bucket must never get the session cookie, and the signed URL is the permission.
    sent = await fetch(ticket.url, { method: ticket.method, headers: ticket.headers, body: file });
  } catch {
    throw new ApiError(0, "The file did not upload. Check your connection and try again.", null);
  }
  if (!sent.ok) throw new ApiError(sent.status, "The file did not upload. Try again.", null);
  return call<Upload>(`/api/v1/uploads/${ticket.id}/complete`, "POST");
}
