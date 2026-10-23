import { expect, test } from "vitest";
import { oauthStartURL, safeReturn, waitText, welcomePath } from "./account";
import { describeDevice, timeAgo } from "./format";

test("safeReturn keeps plain paths and refuses anything that could leave the site", () => {
  expect(safeReturn("/presets/new?x=1")).toBe("/presets/new?x=1");
  for (const bad of [null, "", "presets", "//evil.example", "/\\evil.example", "https://evil.example", "/a\nb", "/" + "a".repeat(200)]) {
    expect(safeReturn(bad)).toBe("/");
  }
});

test("every sign-in goes through onboarding with the final return path", () => {
  expect(welcomePath("/presets")).toBe("/welcome?return=%2Fpresets");
  expect(welcomePath("//evil.example")).toBe("/welcome?return=%2F");
  const url = new URL(oauthStartURL("github", "/welcome?return=%2Fpresets", true));
  expect(url.pathname).toBe("/api/v1/auth/github/start");
  expect(url.searchParams.get("return")).toBe("/welcome?return=%2Fpresets");
  expect(url.searchParams.get("link")).toBe("1");
});

test("waitText rounds the wait up to whole minutes", () => {
  expect(waitText(null)).toBe("later");
  expect(waitText(30)).toBe("in a minute");
  expect(waitText(61)).toBe("in 2 minutes");
});

test("describeDevice names the common browsers and systems", () => {
  const cases: Record<string, string> = {
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36": "Chrome on Windows",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36 Edg/141.0": "Edge on Windows",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": "Safari on iPhone",
    "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0": "Firefox on Linux",
    "bruno-runtime/4.2.1": "Unknown browser",
    "": "Unknown browser",
  };
  for (const [ua, want] of Object.entries(cases)) expect(describeDevice(ua)).toBe(want);
});

test("timeAgo says how long ago in words", () => {
  const now = Date.parse("2026-10-23T12:00:00Z");
  expect(timeAgo("2026-10-23T11:59:30Z", now)).toBe("just now");
  expect(timeAgo("2026-10-23T09:00:00Z", now)).toBe("3 hours ago");
  expect(timeAgo("2026-10-22T12:00:00Z", now)).toBe("yesterday");
});
