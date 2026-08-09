import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

import { requestJson } from "./requestJson";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("requestJson", () => {
  it("parses application/problem+json title", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          type: "forbidden",
          title: "Projection operation forbidden",
          status: 403,
        }),
        {
          status: 403,
          headers: {
            "Content-Type": "application/problem+json",
          },
        },
      ),
    );

    await expect(
      requestJson("/v1/test", {
        method: "GET",
        schema: z.object({}),
      }),
    ).rejects.toMatchObject({
      status: 403,
      message: "Projection operation forbidden",
    });
  });

  it("rejects invalid successful API responses", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          unexpected: true,
        }),
        {
          status: 200,
          headers: {
            "Content-Type": "application/json",
          },
        },
      ),
    );

    await expect(
      requestJson("/v1/test", {
        method: "GET",
        schema: z.object({
          expected: z.string(),
        }),
      }),
    ).rejects.toMatchObject({
      status: 502,
    });
  });
});
