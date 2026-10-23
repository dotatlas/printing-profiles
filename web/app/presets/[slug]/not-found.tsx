import { Suspense } from "react";
import { MissingOrDraft } from "../../../components/preset-pages";

// PresetNotFound shows for a preset path that the server cannot find. The author of a draft still sees the draft,
// because the browser asks the API again with the session. It reads the path, so it waits inside Suspense.
export default function PresetNotFound() {
  return (
    <Suspense>
      <MissingOrDraft />
    </Suspense>
  );
}
