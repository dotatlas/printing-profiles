import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { ApiError } from "../lib/account";
import type { Photo } from "../lib/api-types";
import { PhotoManager } from "./photo-manager";

const addPhoto = vi.fn();
const removePhoto = vi.fn();
vi.mock("../lib/my-presets", () => ({
  addPhoto: (...args: unknown[]) => addPhoto(...args),
  removePhoto: (...args: unknown[]) => removePhoto(...args),
}));

afterEach(() => {
  addPhoto.mockReset();
  removePhoto.mockReset();
});

const photo = (id: string, alt: string): Photo => ({
  id,
  url: `https://files.test/photos/${id}.jpg`,
  thumb_url: `https://files.test/photos/${id}-thumb.jpg`,
  width: 2048,
  height: 1536,
  alt,
});

const jpeg = () => new File([new Uint8Array([0xff, 0xd8, 0xff])], "cube.jpg", { type: "image/jpeg" });

test("the author adds a photo with its description, and the list shows it", async () => {
  addPhoto.mockResolvedValue(photo("bcdefghjkmnp", "The cube from the front"));
  render(<PhotoManager presetId="ab3defgh" initial={[]} />);
  expect(screen.getByText(/No photos yet/)).toBeDefined();

  fireEvent.change(screen.getByLabelText(/^Photo \(/), { target: { files: [jpeg()] } });
  fireEvent.change(screen.getByLabelText("Describe the photo"), { target: { value: "  The cube from the front " } });
  fireEvent.click(screen.getByRole("button", { name: "Add photo" }));

  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Photo added."));
  expect(addPhoto).toHaveBeenCalledWith("ab3defgh", expect.any(File), "The cube from the front");
  expect(screen.getByRole("img", { name: "The cube from the front" })).toBeDefined();
  // The form is empty again for the next photo.
  expect((screen.getByLabelText("Describe the photo") as HTMLInputElement).value).toBe("");
});

test("a missing photo or description is caught before any call, and a refusal says why", async () => {
  render(<PhotoManager presetId="ab3defgh" initial={[]} />);
  fireEvent.click(screen.getByRole("button", { name: "Add photo" }));
  expect(screen.getByText("Choose a photo first.")).toBeDefined();

  fireEvent.change(screen.getByLabelText(/^Photo \(/), { target: { files: [jpeg()] } });
  fireEvent.click(screen.getByRole("button", { name: "Add photo" }));
  expect(screen.getByText("Describe the photo for people who cannot see it.")).toBeDefined();

  const big = new File([new Uint8Array(10 * 1024 * 1024 + 1)], "huge.jpg");
  fireEvent.change(screen.getByLabelText(/^Photo \(/), { target: { files: [big] } });
  fireEvent.change(screen.getByLabelText("Describe the photo"), { target: { value: "Big" } });
  fireEvent.click(screen.getByRole("button", { name: "Add photo" }));
  expect(screen.getByText("The photo is larger than 10 MB. Save a smaller copy and try again.")).toBeDefined();
  expect(addPhoto).not.toHaveBeenCalled();

  addPhoto.mockRejectedValue(new ApiError(422, "this photo format is not supported yet: save it as JPEG or PNG", null));
  fireEvent.change(screen.getByLabelText(/^Photo \(/), { target: { files: [jpeg()] } });
  fireEvent.click(screen.getByRole("button", { name: "Add photo" }));
  expect(await screen.findByText("This photo format is not supported yet: save it as JPEG or PNG.")).toBeDefined();
});

test("Remove asks first, and the focus goes to the heading after the removal", async () => {
  removePhoto.mockResolvedValue(undefined);
  render(<PhotoManager presetId="ab3defgh" initial={[photo("bcdefghjkmnp", "The cube"), photo("cdefghjkmnpq", "The seam")]} />);
  const item = screen.getByRole("img", { name: "The cube" }).closest("li") as HTMLElement;
  fireEvent.click(within(item).getByRole("button", { name: /^Remove/ }));
  expect(document.activeElement?.textContent).toBe("Cancel");
  fireEvent.click(within(item).getByRole("button", { name: "Remove for good" }));
  await waitFor(() => expect(screen.queryByRole("img", { name: "The cube" })).toBeNull());
  expect(removePhoto).toHaveBeenCalledWith("bcdefghjkmnp");
  expect(screen.getByRole("status").textContent).toBe("Photo removed.");
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Photos" }));
});

test("with 10 photos, the form gives way to a note", () => {
  const ten = Array.from({ length: 10 }, (_, i) => photo(`bcdefghjkm${"abcdefghjk"[i]}${"abcdefghjk"[i]}`, `Photo ${i + 1}`));
  render(<PhotoManager presetId="ab3defgh" initial={ten} />);
  expect(screen.queryByRole("button", { name: "Add photo" })).toBeNull();
  expect(screen.getByText("This preset has 10 photos, the most it can have. Remove one to add another.")).toBeDefined();
});

test("a check keeps the focus on Add photo, and an error takes the focus to its field", async () => {
  let finish: (p: Photo) => void = () => {};
  addPhoto.mockImplementation(() => new Promise<Photo>((resolve) => (finish = resolve)));
  render(<PhotoManager presetId="ab3defgh" initial={[]} />);
  const button = screen.getByRole("button", { name: "Add photo" });

  fireEvent.click(button);
  const file = screen.getByLabelText(/^Photo \(/);
  expect(document.activeElement).toBe(file);
  expect(file.getAttribute("aria-describedby")).toContain(screen.getByText("Choose a photo first.").closest("[id]")?.id);

  fireEvent.change(file, { target: { files: [jpeg()] } });
  fireEvent.change(screen.getByLabelText("Describe the photo"), { target: { value: "The cube" } });
  button.focus();
  fireEvent.click(button);
  // While the photo is checked, the button stays focusable and ignores a second press.
  expect(await screen.findByRole("button", { name: "Adding…" })).toBe(document.activeElement);
  fireEvent.click(screen.getByRole("button", { name: "Adding…" }));
  expect(addPhoto).toHaveBeenCalledTimes(1);
  finish(photo("bcdefghjkmnp", "The cube"));
  await waitFor(() => expect(screen.getByRole("status").textContent).toBe("Photo added."));
  expect(screen.getByText("1 of 10 photos")).toBeDefined();
});

test("on the edit page a thumbnail is a plain image, and the section takes the focus when the link asks for it", () => {
  history.replaceState(null, "", "#photos");
  render(<PhotoManager presetId="ab3defgh" initial={[photo("bcdefghjkmnp", "The cube")]} />);
  expect(screen.getByRole("img", { name: "The cube" }).closest("a")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("heading", { name: "Photos" }));
  history.replaceState(null, "", " ");
});
