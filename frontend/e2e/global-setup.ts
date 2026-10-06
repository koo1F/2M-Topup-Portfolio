import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

export default async function globalSetup() {
  const directory = mkdtempSync(path.join(tmpdir(), "2m-topup-worker-"));
  const binary = path.join(directory, "worker");
  execFileSync("go", ["build", "-o", binary, "."], { cwd: path.resolve("../worker"), stdio: "inherit" });
  const worker = spawn(binary, [], {
    env: {
      ...process.env,
      DATABASE_URL: process.env.E2E_DATABASE_URL ?? "postgres://user:password@127.0.0.1:55432/payment_test?sslmode=disable",
      REDIS_URL: process.env.E2E_REDIS_URL ?? "127.0.0.1:56379",
    },
    stdio: "inherit",
  });
  return async () => {
    await new Promise<void>((resolve) => {
      if (worker.exitCode !== null) { resolve(); return; }
      const timer = setTimeout(() => worker.kill("SIGKILL"), 10_000);
      worker.once("exit", () => { clearTimeout(timer); resolve(); });
      worker.kill("SIGTERM");
    });
    rmSync(directory, { recursive: true, force: true });
  };
}
