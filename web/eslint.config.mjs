import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

export default defineConfig([
  ...nextVitals,
  ...nextTs,
  // The default ignores of eslint-config-next, and the generated API types.
  globalIgnores([".next/**", "out/**", "build/**", "next-env.d.ts", "lib/api-types.ts"]),
]);
