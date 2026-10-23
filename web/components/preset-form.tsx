"use client";

import { type FormEvent, type ReactNode, useId, useState } from "react";
import { ApiError, waitText } from "../lib/account";
import { type License, licenseValues, nozzleValues, type Preset } from "../lib/api-types";
import { createPreset, type FormField, fieldOfProblem, updatePreset } from "../lib/my-presets";
import { nozzleLabel } from "../lib/presets";
import { Notice } from "./account-ui";
import { type Catalog, MaterialOptions, PrinterOptions } from "./preset-list";

// licenseText names each license and says in one line what it lets other people do.
const licenseText: Record<License, { name: string; text: string }> = {
  "CC-BY-4.0": { name: "CC BY 4.0", text: "Anyone may share and change it, and must name you." },
  "CC-BY-SA-4.0": { name: "CC BY-SA 4.0", text: "The same, and changed copies must keep this license." },
  "CC-BY-NC-4.0": { name: "CC BY-NC 4.0", text: "The same as CC BY, but not for commercial use." },
  "CC0-1.0": { name: "CC0 1.0", text: "No conditions: you give up your rights to it." },
};

const maxDescription = 4000;

type Values = Record<"title" | "printer" | "material" | "nozzle" | "slicer" | "slicerVersion" | "description", string> & {
  license: License;
};
type Errors = Partial<Record<FormField, string>>;

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
  const title = v.title.trim().length;
  if (title < 5 || title > 100) errors.title = title === 0 ? "Give the preset a title." : "The title needs 5 to 100 characters.";
  if (!v.printer) errors.printer = "Choose a printer.";
  if (!v.material) errors.material = "Choose a filament.";
  if (!v.slicer) errors.slicer = "Choose a slicer.";
  return errors;
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
// gives the preset. It calls onSaved with the API's answer, and the page decides where to go next.
export function PresetForm({ catalog, preset, onSaved }: { catalog: Catalog; preset?: Preset; onSaved: (p: Preset) => void }) {
  const base = useId();
  const [values, setValues] = useState(() => valuesOf(preset));
  const [errors, setErrors] = useState<Errors>({});
  const [problem, setProblem] = useState("");
  const [saving, setSaving] = useState<"draft" | "published" | null>(null);
  const draft = !preset || preset.status === "draft";

  const set = (key: keyof Values) => (e: { target: { value: string } }) => setValues((v) => ({ ...v, [key]: e.target.value }));
  const invalid = (field: FormField) => (errors[field] ? true : undefined);

  // showErrors puts the messages under their fields and moves the focus to the first one, as a screen reader needs.
  function showErrors(next: Errors) {
    setErrors(next);
    const first = (Object.keys(next) as FormField[])[0];
    if (first) document.getElementById(idOf(base, first))?.focus();
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
    };
    try {
      onSaved(preset ? await updatePreset(preset.id, input) : await createPreset(input));
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

      <Field
        id={idOf(base, "description")}
        label="Description (optional)"
        hint={left < 500 ? `${left} characters left.` : "What did you tune, and what does it print well? Plain text."}
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
              <button type="submit" value="published" className="btn btn-primary" disabled={saving !== null}>
                {saving === "published" ? "Publishing…" : "Publish"}
              </button>
              <button type="submit" value="draft" className="btn btn-secondary" disabled={saving !== null}>
                {saving === "draft" ? "Saving…" : preset ? "Save draft" : "Save as draft"}
              </button>
            </>
          ) : (
            <button type="submit" value="published" className="btn btn-primary" disabled={saving !== null}>
              {saving ? "Saving…" : "Save changes"}
            </button>
          )}
        </div>
        {draft && <p className="text-sm text-muted">A draft is visible only to you.</p>}
      </div>
    </form>
  );
}
