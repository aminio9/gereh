import { z } from "zod";

import { requestJson } from "../../../lib/http/requestJson";

export const systemStatusSchema = z.object({
  status: z.literal("ok"),
  service: z.string().min(1),
  version: z.string().min(1),
});

export type SystemStatus = z.infer<typeof systemStatusSchema>;

export async function getSystemStatus(): Promise<SystemStatus> {
  return requestJson("/v1/status", {
    method: "GET",
    schema: systemStatusSchema,
    timeoutMs: 5_000,
  });
}
