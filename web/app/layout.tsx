import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";
import { themeScript } from "../components/theme";
import { ThemeSwitch } from "../components/theme-switch";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "Printing Profiles", template: "%s · Printing Profiles" },
  description: "Slicer presets for your printer and filament, shared by the 3D printing community.",
};

// RootLayout is the frame of every page: the header, the page content and the footer. Next.js puts each page in it.
export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    // The theme script changes data-theme before React starts, so React must not report the difference.
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="flex min-h-dvh flex-col bg-page text-ink">
        <header className="border-b border-line">
          <div className="mx-auto flex max-w-3xl items-center justify-between gap-4 px-4 py-3">
            <Link href="/" className="font-semibold text-ink no-underline">
              Printing Profiles
            </Link>
            <ThemeSwitch />
          </div>
        </header>
        <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-10">{children}</main>
        <footer className="border-t border-line">
          <p className="mx-auto max-w-3xl px-4 py-6 text-sm text-muted">
            Open source under the AGPL-3.0 license ·{" "}
            <a href="https://github.com/dotatlas/printing-profiles">Source code</a>
          </p>
        </footer>
      </body>
    </html>
  );
}
