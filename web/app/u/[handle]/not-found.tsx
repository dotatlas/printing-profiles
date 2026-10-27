import Link from "next/link";

// ProfileNotFound shows for a profile path whose handle no member has. Next.js sends it with status 404.
export default function ProfileNotFound() {
  return (
    <>
      <h1 className="text-2xl font-semibold tracking-tight">Member not found</h1>
      <p className="mt-2 text-muted">No member has this handle. Check the link, or find presets by setup.</p>
      <p className="mt-6">
        <Link href="/presets">Browse presets</Link>
      </p>
    </>
  );
}
