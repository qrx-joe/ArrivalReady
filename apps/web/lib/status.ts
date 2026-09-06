/**
 * Pure status aggregation shared by future health widgets.
 * Kept dependency-free so the smoke test proves the test pipeline itself.
 */
export type ServiceProbe = {
  name: string;
  status: "ok" | "unavailable";
};

export function overallStatus(probes: ServiceProbe[]): "ok" | "degraded" {
  return probes.every((p) => p.status === "ok") ? "ok" : "degraded";
}
