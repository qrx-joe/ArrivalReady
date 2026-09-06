import { describe, expect, it } from "vitest";

import { overallStatus } from "./status";

describe("overallStatus", () => {
  it("reports ok only when every service is ok", () => {
    expect(
      overallStatus([
        { name: "api", status: "ok" },
        { name: "ai", status: "ok" },
      ]),
    ).toBe("ok");
  });

  it("degrades when any service is unavailable", () => {
    expect(
      overallStatus([
        { name: "api", status: "ok" },
        { name: "ai", status: "unavailable" },
      ]),
    ).toBe("degraded");
  });
});
