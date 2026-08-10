import { queryOptions } from "@tanstack/react-query";
import { z } from "zod";

import { requestJson } from "../../../lib/http/requestJson";

const protoIntegerSchema = z.coerce.number().int().nonnegative();

export const tenantSchema = z.object({
  tenantId: z.string().min(1),
  slug: z.string().default(""),
  displayName: z.string().default(""),
  status: z.string().default("TENANT_STATUS_UNSPECIFIED"),
  region: z.string().default(""),
  retentionDays: z.coerce.number().int().nonnegative().optional().default(0),
  version: protoIntegerSchema.optional().default(0),
  createdByUserId: z.string().optional().default(""),
  createdAt: z.string().optional(),
  updatedAt: z.string().optional(),
  archivedAt: z.string().optional(),
});

export const tenantContextSchema = z
  .object({
    tenant: tenantSchema,
    permissions: z.array(z.string()).optional().default([]),
  })
  .passthrough();

const listTenantsResponseSchema = z.object({
  contexts: z.array(tenantContextSchema).optional().default([]),
  nextPageToken: z.string().optional().default(""),
});

export type Tenant = z.infer<typeof tenantSchema>;
export type TenantContext = z.infer<typeof tenantContextSchema>;
export type ListTenantsResponse = z.infer<typeof listTenantsResponseSchema>;

export const tenantQueryKeys = {
  all: ["tenants"] as const,
};

export async function listTenants(signal?: AbortSignal): Promise<ListTenantsResponse> {
  return requestJson("/v1/tenants/?page_size=50", {
    method: "GET",
    schema: listTenantsResponseSchema,
    signal,
  });
}

export function tenantsQueryOptions(enabled = true) {
  return queryOptions({
    queryKey: tenantQueryKeys.all,
    queryFn: ({ signal }) => listTenants(signal),
    enabled,
    staleTime: 60_000,
  });
}
