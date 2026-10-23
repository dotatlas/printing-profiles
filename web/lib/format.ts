// describeDevice turns a browser's user agent into words, such as "Firefox on Windows". The sessions list uses it.
// shortcut: it knows the common browsers and systems only; anything else reads "Unknown browser".
export function describeDevice(userAgent: string): string {
  const ua = userAgent || "";
  const browser = /Edg\//.test(ua)
    ? "Edge"
    : /OPR\//.test(ua)
      ? "Opera"
      : /Firefox\//.test(ua)
        ? "Firefox"
        : /Chrome\//.test(ua)
          ? "Chrome"
          : /Safari\//.test(ua)
            ? "Safari"
            : "";
  const system = /iPhone/.test(ua)
    ? "iPhone"
    : /iPad/.test(ua)
      ? "iPad"
      : /Android/.test(ua)
        ? "Android"
        : /Windows/.test(ua)
          ? "Windows"
          : /Mac OS X|Macintosh/.test(ua)
            ? "macOS"
            : /Linux/.test(ua)
              ? "Linux"
              : "";
  if (browser && system) return `${browser} on ${system}`;
  return browser || system || "Unknown browser";
}

const dateFormat = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric" });

// formatDate writes a date like "23 Oct 2026". The account settings use it.
export function formatDate(iso: string): string {
  return dateFormat.format(new Date(iso));
}

const relative = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

// timeAgo writes how long ago a time was, such as "3 hours ago" or "yesterday". The sessions list uses it.
export function timeAgo(iso: string, now = Date.now()): string {
  const seconds = Math.round((new Date(iso).getTime() - now) / 1000);
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ["day", 86400],
    ["hour", 3600],
    ["minute", 60],
  ];
  for (const [unit, size] of steps) {
    if (Math.abs(seconds) >= size) return relative.format(Math.round(seconds / size), unit);
  }
  return "just now";
}
