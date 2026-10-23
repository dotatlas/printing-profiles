import type { NextConfig } from "next";

// The site sends these headers with every response. The API sends its own headers.
const securityHeaders = [
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Content-Security-Policy", value: "frame-ancestors 'none'" },
];

const nextConfig: NextConfig = {
  output: "standalone",
  // The browser calls the API for sign-in and account data. Its URL is built in: NEXT_PUBLIC_API_URL when it is
  // set (a self-hosted site whose server reads the API on an internal address), otherwise API_URL.
  env: { NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL ?? process.env.API_URL ?? "http://localhost:8080" },
  poweredByHeader: false,
  // The images are already small. The server sends them as they are and writes no image cache to the read-only disk.
  images: { unoptimized: true },
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  // headers gives the security headers for every path. Next.js reads it at build time.
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
