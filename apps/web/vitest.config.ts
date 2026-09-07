import { defineConfig } from "vitest/config";

// tests/e2e is Playwright territory (browser tests against the live stack);
// Vitest owns only the unit specs.
export default defineConfig({
  test: {
    include: ["lib/**/*.test.ts", "app/**/*.test.tsx", "app/**/*.test.ts"],
    exclude: ["tests/e2e/**", "node_modules/**"],
  },
});
