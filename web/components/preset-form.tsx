"use client";

import Link from "next/link";
import { type ChangeEvent, type FormEvent, type ReactNode, useEffect, useId, useState } from "react";
import { ApiError, waitText } from "../lib/account";
import { type KeySetting, type License, licenseValues, nozzleValues, type Preset } from "../lib/api-types";
import { createPreset, type FormField, fieldOfProblem, updatePreset, uploadPresetFile } from "../lib/my-presets";
import { nozzleLabel } from "../lib/presets";
import { Notice } from "./account-ui";
import { fileLabel, KeySettings } from "./preset-facts";
import { type Catalog, MaterialOptions, PrinterOptions } from "./preset-list";

// licenseText names each license and says in one line what it lets other people do.
const licenseText: Record<License, { name: string; text: string }> = {
  "CC-BY-4.0": { name: "CC BY 4.0", text: "Anyone may share and change it, and must name you." },
  "CC-BY-SA-4.0": { name: "CC BY-SA 4.0", text: "The same, and changed copies must keep this license." },
  "CC-BY-NC-4.0": { name: "CC BY-NC 4.0", text: "The same as CC BY, but not for commercial use." },
  "CC0-1.0": { name: "CC0 1.0", text: "No conditions: you give up your rights to it." },
};

const maxDescription = 4000;
const maxFile = 1 << 20; // the API's limit for a preset file

type Values = Record<"title" | "printer" | "material" | "nozzle" | "slicer" | "slicerVersion" | "description", string> & {
  license: License;
};
type Errors = Partial<Record<FormField, string>>;

// CheckedFile is what the form shows for a preset file: a new upload has an id; the edited preset's file has none.
type CheckedFile = { id?: string; slicer: string; kind: string; slicer_version?: string; settings?: KeySetting[]; removed?: string[] };
type FileState = { status: "none" } | { status: "checking" } | { status: "ready"; file: CheckedFile };

// slicerOfFile gives the catalog slicer that a checked file fills in: only PrusaSlicer and SuperSlicer files say
// which slicer they are; the OrcaSlicer format is shared with Bambu Studio and Creality Print.
const slicerOfFile: Record<string, string> = { prusaslicer: "prusaslicer", superslicer: "superslicer" };

// valuesOf gives the start values of the form: the preset's own values, or empty choices with the usual nozzle.
function valuesOf(preset?: Preset): Values {
  return {
    title: preset?.title ?? "",
    printer: preset?.printer.slug ?? "",
    material: preset?.material.slug ?? "",
    nozzle: String(preset?.nozzle_mm ?? 0.4),
    slicer: preset?.slicer.slug ?? "",
    slicerVersion: preset?.slicer_version ?? "",
    description: preset?.description ?? "",
    license: preset?.license ?? "CC-BY-4.0",
  };
}

// checkValues finds the fields that the API would refuse, in form order, so the form can answer at once.
function checkValues(v: Values): Errors {
  const errors: Errors = {};
  if (!v.printer) errors.printer = "Choose a printer.";
  if (!v.material) errors.material = "Choose a filament.";
  if (!v.slicer) errors.slicer = "Choose a slicer.";
  const title = v.title.trim().length;
  if (title === 0) errors.title = "Give the preset a title.";
  else if (title < 5) errors.title = `The title needs at least 5 characters; it has ${title}.`;
  return errors;
}

// keptKey is where the new form keeps what the visitor typed, so a reload or a wrong link loses nothing.
const keptKey = "preset-form:new";

// keptValues reads what the new form kept in this tab, or null. Storage can be missing or blocked.
function keptValues(): Values | null {
  try {
    const text = sessionStorage.getItem(keptKey);
    return text ? { ...valuesOf(), ...(JSON.parse(text) as Partial<Values>) } : null;
  } catch {
    return null;
  }
}

// keep writes the new form's values to this tab's storage, or removes them with null.
function keep(values: Values | null) {
  try {
    if (values) sessionStorage.setItem(keptKey, JSON.stringify(values));
    else sessionStorage.removeItem(keptKey);
  } catch {
    // Without storage the form still works; it only cannot keep the values.
  }
}

// problemText says why a save failed when the error is not about one field. PresetForm shows it above the buttons.
function problemText(err: ApiError | null): string {
  if (err?.status === 0) return err.detail;
  if (err?.status === 429) return `You shared many presets in the last hour. Try again ${waitText(err.retryAfter)}.`;
  if (err && err.status < 500 && err.detail) return `The preset was not saved: ${err.detail}.`;
  return "The preset was not saved. Try again in a moment.";
}

// idOf gives the element ID of a form field, so the form can move the focus to the first field with an error.
function idOf(base: string, field: FormField): string {
  return `${base}-${field}`;
}

// Field is one labelled control with its hint and its error. The error replaces the hint and is read with the field.
function Field({
  id,
  label,
  hint,
  error,
  children,
}: {
  id: string;
  label: string;
  hint?: ReactNode;
  error?: string;
  children: (describedBy: string | undefined) => ReactNode;
}) {
  const note = error ?? hint;
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children(note ? `${id}-note` : undefined)}
      {note && (
        <p id={`${id}-note`} className={error ? "text-sm text-danger" : "text-sm text-muted"}>
          {note}
        </p>
      )}
    </div>
  );
}

// PresetForm makes a new preset or edits one. The new and edit pages give it the catalog, and the edit page also
// gives the preset and the way back (cancelHref). It calls onSaved with the API's answer, and the page decides where
// to go next. The new form keeps the typed values in this tab until a save.
export function PresetForm({
  catalog,
  preset,
  cancelHref,
  onSaved,
}: {
  catalog: Catalog;
  preset?: Preset;
  cancelHref?: string;
  onSaved: (p: Preset) => void;
}) {
  const base = useId();
  // The form renders only in the browser (the pages wait for the session first), so storage is there to read.
  const [kept] = useState(() => (preset ? null : keptValues()));
  const [values, setValues] = useState(() => kept ?? valuesOf(preset));
  const [restored, setRestored] = useState(kept !== null);
  const [errors, setErrors] = useState<Errors>({});
  const [problem, setProblem] = useState("");
  const [saving, setSaving] = useState<"draft" | "published" | null>(null);
  const [file, setFile] = useState<FileState>(() => (preset?.file ? { status: "ready", file: preset.file } : { status: "none" }));
  const draft = !preset || preset.status === "draft";

  useEffect(() => {
    if (!preset) keep(values);
  }, [preset, values]);

  const set = (key: keyof Values) => (e: { target: { value: string } }) => setValues((v) => ({ ...v, [key]: e.target.value }));
  const invalid = (field: FormField) => (errors[field] ? true : undefined);

  // showErrors puts the messages under their fields and moves the focus to the first one, as a screen reader needs.
  function showErrors(next: Errors) {
    setErrors(next);
    const first = (Object.keys(next) as FormField[])[0];
    if (first) document.getElementById(idOf(base, first))?.focus();
  }

  // onFile sends the chosen file to storage, has the API check it, and shows what the check found. A PrusaSlicer or
  // SuperSlicer file fills in an empty slicer.
  async function onFile(e: ChangeEvent<HTMLInputElement>) {
    const chosen = e.target.files?.[0];
    if (!chosen) return;
    setErrors((old) => ({ ...old, file: undefined }));
    if (chosen.size === 0 || chosen.size > maxFile) {
      setFile({ status: "none" });
      const reason = chosen.size === 0 ? "The file is empty." : "The file is larger than 1 MB, so it is not a preset file.";
      setErrors((old) => ({ ...old, file: reason }));
      return;
    }
    setFile({ status: "checking" });
    try {
      const checked = await uploadPresetFile(chosen);
      setFile({ status: "ready", file: { ...checked, slicer: checked.slicer ?? "", kind: checked.kind ?? "" } });
      const slug = slicerOfFile[checked.slicer ?? ""];
      if (slug && catalog.slicers.some((s) => s.slug === slug)) setValues((v) => (v.slicer ? v : { ...v, slicer: slug }));
    } catch (err) {
      setFile({ status: "none" });
      const reason = err instanceof ApiError && err.detail ? err.detail : "The file was not checked. Try again.";
      setErrors((old) => ({ ...old, file: reason.charAt(0).toUpperCase() + reason.slice(1) }));
    }
  }

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    // The first submit button is the default one, so Enter in a field does what the primary button does.
    const submitter = (e.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    const status: "draft" | "published" = submitter?.value === "draft" ? "draft" : "published";
    setProblem("");
    const found = checkValues(values);
    if (Object.keys(found).length > 0) return showErrors(found);
    setErrors({});
    setSaving(status);
    const input = {
      title: values.title.trim(),
      description: values.description,
      printer: values.printer,
      material: values.material,
      nozzle_mm: Number(values.nozzle) as Preset["nozzle_mm"],
      slicer: values.slicer,
      slicer_version: values.slicerVersion.trim(),
      license: values.license,
      status,
      // Only a new upload goes with the preset; the edited preset keeps its file otherwise.
      ...(file.status === "ready" && file.file.id ? { file: file.file.id } : {}),
    };
    try {
      const saved = preset ? await updatePreset(preset.id, input) : await createPreset(input);
      if (!preset) keep(null);
      onSaved(saved);
    } catch (err) {
      const api = err instanceof ApiError ? err : null;
      const field = api?.status === 400 ? fieldOfProblem(api.detail) : null;
      if (api && field) showErrors({ [field]: api.detail });
      else setProblem(problemText(api));
    } finally {
      setSaving(null);
    }
  }

  const left = maxDescription - values.description.length;
  return (
    <form onSubmit={onSubmit} noValidate className="flex max-w-2xl flex-col gap-8">
      {restored && (
        <Notice>
          This form kept what you typed before.{" "}
          <button
            type="button"
            className="underline underline-offset-2"
            onClick={() => {
              setValues(valuesOf());
              setRestored(false);
            }}
          >
            Start over
          </button>
        </Notice>
      )}
      <Field
        id={idOf(base, "file")}
        label="Preset file (optional)"
        // Once a file is checked, its panel says what it is, so the hint goes away.
        hint={file.status === "ready" ? undefined : "PrusaSlicer or SuperSlicer .ini, OrcaSlicer or Bambu Studio .json, up to 1 MB. You can add it later."}
        error={errors.file}
      >
        {(describedBy) => (
          <>
            <input
              id={idOf(base, "file")}
              type="file"
              accept=".ini,.json"
              className="file-input"
              onChange={onFile}
              disabled={file.status === "checking"}
              aria-invalid={invalid("file")}
              aria-describedby={describedBy}
            />
            {file.status === "checking" && (
              <p role="status" className="text-sm text-muted">
                Checking the file…
              </p>
            )}
            {file.status === "ready" && (
              <div className="panel mt-1 flex flex-col gap-3">
                <p className="text-sm font-medium">{fileLabel(file.file)}</p>
                {file.file.settings && <KeySettings settings={file.file.settings} />}
                {file.file.removed && file.file.removed.length > 0 && (
                  <Notice>
                    The file held your printer&apos;s network settings, so they were removed before it was stored:{" "}
                    {file.file.removed.join(", ")}.
                  </Notice>
                )}
              </div>
            )}
          </>
        )}
      </Field>

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-3 font-semibold">Your setup</legend>
        <Field id={idOf(base, "printer")} label="Printer" error={errors.printer}>
          {(describedBy) => (
            <select id={idOf(base, "printer")} className="field" value={values.printer} onChange={set("printer")} aria-invalid={invalid("printer")} aria-describedby={describedBy}>
              <option value="">Choose a printer</option>
              <PrinterOptions printers={catalog.printers} />
            </select>
          )}
        </Field>
        <Field id={idOf(base, "material")} label="Filament" error={errors.material}>
          {(describedBy) => (
            <select id={idOf(base, "material")} className="field" value={values.material} onChange={set("material")} aria-invalid={invalid("material")} aria-describedby={describedBy}>
              <option value="">Choose a filament</option>
              <MaterialOptions materials={catalog.materials} />
            </select>
          )}
        </Field>
        <Field id={idOf(base, "nozzle_mm")} label="Nozzle" error={errors.nozzle_mm}>
          {(describedBy) => (
            <select id={idOf(base, "nozzle_mm")} className="field" value={values.nozzle} onChange={set("nozzle")} aria-invalid={invalid("nozzle_mm")} aria-describedby={describedBy}>
              {nozzleValues.map((mm) => (
                <option key={mm} value={String(mm)}>
                  {nozzleLabel(mm)}
                </option>
              ))}
            </select>
          )}
        </Field>
        <Field id={idOf(base, "slicer")} label="Slicer" error={errors.slicer}>
          {(describedBy) => (
            <select id={idOf(base, "slicer")} className="field" value={values.slicer} onChange={set("slicer")} aria-invalid={invalid("slicer")} aria-describedby={describedBy}>
              <option value="">Choose a slicer</option>
              {catalog.slicers.map((s) => (
                <option key={s.slug} value={s.slug}>
                  {s.name}
                </option>
              ))}
            </select>
          )}
        </Field>
        <Field id={idOf(base, "slicer_version")} label="Slicer version (optional)" hint="As the slicer shows it, for example 2.3.0." error={errors.slicer_version}>
          {(describedBy) => (
            <input
              id={idOf(base, "slicer_version")}
              className="field"
              value={values.slicerVersion}
              onChange={set("slicerVersion")}
              maxLength={20}
              autoComplete="off"
              aria-invalid={invalid("slicer_version")}
              aria-describedby={describedBy}
            />
          )}
        </Field>
      </fieldset>

      <Field id={idOf(base, "title")} label="Title" hint="5 to 100 characters. Say what the preset is good at." error={errors.title}>
        {(describedBy) => (
          <input
            id={idOf(base, "title")}
            className="field"
            value={values.title}
            onChange={set("title")}
            maxLength={100}
            autoComplete="off"
            aria-invalid={invalid("title")}
            aria-describedby={describedBy}
          />
        )}
      </Field>

      <Field
        id={idOf(base, "description")}
        label="Description (optional)"
        hint={left < 500 ? `${left} ${left === 1 ? "character" : "characters"} left.` : "What did you tune, and what does it print well? Plain text."}
        error={errors.description}
      >
        {(describedBy) => (
          <textarea
            id={idOf(base, "description")}
            className="field h-auto min-h-40 py-2"
            value={values.description}
            onChange={set("description")}
            maxLength={maxDescription}
            aria-invalid={invalid("description")}
            aria-describedby={describedBy}
          />
        )}
      </Field>

      <fieldset>
        <legend className="font-semibold">License</legend>
        <p className="mt-1 text-sm text-muted">What other people may do with your preset.</p>
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          {licenseValues.map((license) => (
            <label key={license} className="license-choice">
              <input
                type="radio"
                name={`${base}-license`}
                value={license}
                checked={values.license === license}
                onChange={() => setValues((v) => ({ ...v, license }))}
                className="mt-1 accent-accent focus-visible:outline-none"
              />
              <span>
                <span className="block text-sm font-medium">{licenseText[license].name}</span>
                <span className="block text-sm text-muted">{licenseText[license].text}</span>
              </span>
            </label>
          ))}
        </div>
      </fieldset>

      <div className="flex flex-col gap-4">
        {problem && <Notice tone="error">{problem}</Notice>}
        <div className="flex flex-wrap gap-3">
          {draft ? (
            <>
              <button type="submit" value="published" className="btn btn-primary" disabled={saving !== null || file.status === "checking"}>
                {saving === "published" ? "Publishing…" : "Publish"}
              </button>
              <button type="submit" value="draft" className="btn btn-secondary" disabled={saving !== null || file.status === "checking"}>
                {saving === "draft" ? "Saving…" : "Save as draft"}
              </button>
            </>
          ) : (
            <button type="submit" value="published" className="btn btn-primary" disabled={saving !== null || file.status === "checking"}>
              {saving ? "Saving…" : "Save changes"}
            </button>
          )}
          {cancelHref && (
            <Link href={cancelHref} className="btn btn-secondary">
              Cancel
            </Link>
          )}
        </div>
        <p className="text-sm text-muted">
          {draft ? "A draft is visible only to you." : "Saved changes show on the public page at once."}
        </p>
      </div>
    </form>
  );
}
