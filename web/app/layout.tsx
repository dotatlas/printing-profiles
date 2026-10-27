import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import { type ReactNode, Suspense } from "react";
import { AccountMenu } from "../components/account-menu";
import { SessionProvider } from "../components/session";
import { themeScript } from "../components/theme";
import { ThemeSwitch } from "../components/theme-switch";
import "./globals.css";
import logo from "./logo.png";

export const metadata: Metadata = {
  // Link previews need absolute URLs. The build reads SITE_URL, so set it when you build for a real domain.
  metadataBase: new URL(process.env.SITE_URL ?? "http://localhost:3000"),
  title: { default: "Printing Profiles", template: "%s · Printing Profiles" },
  description: "Slicer presets for your printer and filament, shared by the 3D printing community.",
};

// RootLayout is the frame of every page: the header, the page content and the footer with the theme switch.
// Next.js puts each page in it.
export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    // The theme script changes data-theme before React starts, so React must not report the difference.
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="flex min-h-dvh flex-col bg-page text-ink">
        <SessionProvider>
          <header className="border-b border-line">
            {/* The row wraps when a reader sets very large text, so the account control never leaves the screen. */}
            <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-3">
              <Link href="/" className="flex shrink-0 items-center gap-2 font-semibold text-ink no-underline">
                {/* The link text names the site, so the logo has an empty alt text. On a phone the name is for screen
                    readers only, so the presets link and the account control fit beside the logo. */}
                <Image src={logo} alt="" className="size-8" loading="eager" />
                <span className="sr-only sm:not-sr-only">Printing Profiles</span>
              </Link>
              <nav aria-label="Main" className="mr-auto">
                <Link href="/presets" className="rounded-lg px-2 py-2 text-sm font-medium text-ink no-underline hover:bg-raised">
                  Presets
                </Link>
              </nav>
              <div className="flex items-center gap-2">
                {/* The menu reads the path. A page whose path is known only at request time gets the menu's
                    loading space in its prerendered shell. */}
                <Suspense fallback={<div className="h-10 w-20" aria-hidden="true" />}>
                  <AccountMenu />
                </Suspense>
              </div>
            </div>
          </header>
          <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-10">{children}</main>
          <footer className="border-t border-line">
            {/* The theme switch is a rare setting, so it lives here and leaves the header room on a phone. */}
            <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 px-4 py-6">
              <p className="text-sm text-muted">
                Open source under the AGPL-3.0 license ·{" "}
                <a href="https://github.com/dotatlas/printing-profiles">Source code</a>
              </p>
              <ThemeSwitch />
            </div>
          </footer>
        </SessionProvider>
      </body>
    </html>
  );
}
