import { describe, expect, it } from "vitest";

import { agentOverviewResponseSchema, dashboardResponseSchema } from "./schemas";

describe("Projection response schemas", () => {
  it("treats protobuf zero counters as zero", () => {
    const result = dashboardResponseSchema.parse({
      summary: {},
      metadata: {},
    });

    expect(result.summary.agentsTotal).toBe(0);
    expect(result.summary.tasksTotal).toBe(0);
    expect(result.summary.tasksBlocked).toBe(0);
    expect(result.summary.projectsActive).toBe(0);
  });

  it("coerces protobuf int64 JSON strings into numbers", () => {
    const result = dashboardResponseSchema.parse({
      summary: {
        agentsTotal: "12",
        agentsReady: "8",
        tasksTotal: "42",
      },
    });

    expect(result.summary.agentsTotal).toBe(12);
    expect(result.summary.agentsReady).toBe(8);
    expect(result.summary.tasksTotal).toBe(42);
  });

  it("accepts an empty repeated agents field", () => {
    const result = agentOverviewResponseSchema.parse({});

    expect(result.agents).toEqual([]);
    expect(result.nextPageToken).toBe("");
  });
});
