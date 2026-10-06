import { test } from "@playwright/test";
import { execFileSync } from "node:child_process";

// This integration test does not launch a browser and can also run in a
// restricted environment; the UI test separately verifies browser behaviour.
test("HTTP wallet flow through frontend, SQL, Redis and worker", () => {
  execFileSync(process.execPath, ["../scripts/verify-flow.mjs"], { stdio: "inherit", timeout: 60_000 });
});
