import type { Metadata } from "next";
import { EmailLink } from "../../../components/email-link";

export const metadata: Metadata = { title: "Finish signing in", robots: { index: false } };

// EmailLinkPage is where a sign-in email's link opens. The token stays in the browser, so the page renders there.
export default function EmailLinkPage() {
  return <EmailLink />;
}
