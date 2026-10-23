"use client";

import { type ReactNode, useSyncExternalStore } from "react";
import { themeKey } from "./theme";

type Theme = "system" | "light" | "dark";

// readTheme gives the theme that the <html> element shows now. ThemeSwitch uses it to mark the active button.
function readTheme(): Theme {
  const theme = document.documentElement.dataset.theme;
  return theme === "light" || theme === "dark" ? theme : "system";
}

// watchTheme calls onChange when data-theme on <html> changes. React uses it to update ThemeSwitch.
function watchTheme(onChange: () => void) {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  return () => observer.disconnect();
}

// applyTheme shows the theme and saves the choice. "system" removes the choice, so the OS setting applies again.
function applyTheme(theme: Theme) {
  try {
    if (theme === "system") localStorage.removeItem(themeKey);
    else localStorage.setItem(themeKey, theme);
  } catch {
    // The browser can block storage. Then the choice applies only until the page reloads.
  }
  if (theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
}

const iconProps = {
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round",
  strokeLinejoin: "round",
  className: "size-4",
  "aria-hidden": true,
} as const;

const choices: { theme: Theme; label: string; icon: ReactNode }[] = [
  {
    theme: "system",
    label: "System theme",
    icon: (
      <svg {...iconProps}>
        <rect x="3" y="4" width="18" height="12" rx="1" />
        <path d="M8 20h8M12 16v4" />
      </svg>
    ),
  },
  {
    theme: "light",
    label: "Light theme",
    icon: (
      <svg {...iconProps}>
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
      </svg>
    ),
  },
  {
    theme: "dark",
    label: "Dark theme",
    icon: (
      <svg {...iconProps}>
        <path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" />
      </svg>
    ),
  },
];

// ThemeSwitch shows three buttons: system, light and dark. The layout puts it in the header.
export function ThemeSwitch() {
  // The server does not know the choice, so the first HTML marks "system". The browser then marks the real choice.
  const current = useSyncExternalStore(watchTheme, readTheme, (): Theme => "system");
  return (
    <div role="group" aria-label="Theme" className="flex gap-0.5 rounded-md border border-line p-0.5">
      {choices.map(({ theme, label, icon }) => (
        <button
          key={theme}
          type="button"
          title={label}
          aria-label={label}
          aria-pressed={current === theme}
          onClick={() => applyTheme(theme)}
          className="rounded p-1.5 text-muted hover:text-ink aria-pressed:bg-line aria-pressed:text-ink sm:p-2"
        >
          {icon}
        </button>
      ))}
    </div>
  );
}
