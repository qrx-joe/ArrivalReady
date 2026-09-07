import { defineConfig } from "@playwright/test";

// E2E assumes the full local stack is already running (see scripts/dev-e2e.ps1):
// Go API :8080, AI service :8100 (ARRIVAL_FAKE_MODEL=1), Web :3000.
export default defineConfig({
  testDir: "./tests/e2e",
  timeout: 120_000,
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    trace: "retain-on-failure",
  },
  workers: 1, // shared throwaway backend; serial for determinism
});
