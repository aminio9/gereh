import { queryOptions } from "@tanstack/react-query";

import { requestJson } from "../../../lib/http/requestJson";
import {
  activityResponseSchema,
  agentOverviewResponseSchema,
  dashboardResponseSchema,
  searchResponseSchema,
  type ActivityResponse,
  type AgentOverviewResponse,
  type DashboardResponse,
  type SearchResponse,
} from "../schemas";

function tenantPath(tenantId: string, suffix: string): string {
  return `/v1/tenants/${encodeURIComponent(tenantId)}${suffix}`;
}

export async function getDashboard(
  tenantId: string,
  signal?: AbortSignal,
): Promise<DashboardResponse> {
  return requestJson(tenantPath(tenantId, "/dashboard"), {
    method: "GET",
    schema: dashboardResponseSchema,
    signal,
  });
}

export async function listAgentOverviews(
  tenantId: string,
  pageSize: number,
  signal?: AbortSignal,
): Promise<AgentOverviewResponse> {
  const parameters = new URLSearchParams({
    page_size: String(pageSize),
  });

  return requestJson(tenantPath(tenantId, `/agents/overview?${parameters.toString()}`), {
    method: "GET",
    schema: agentOverviewResponseSchema,
    signal,
  });
}

export async function listActivity(
  tenantId: string,
  pageSize: number,
  signal?: AbortSignal,
): Promise<ActivityResponse> {
  const parameters = new URLSearchParams({
    page_size: String(pageSize),
  });

  return requestJson(tenantPath(tenantId, `/activity?${parameters.toString()}`), {
    method: "GET",
    schema: activityResponseSchema,
    signal,
  });
}

export async function searchProjection(
  tenantId: string,
  query: string,
  signal?: AbortSignal,
): Promise<SearchResponse> {
  const parameters = new URLSearchParams({
    q: query,
    page_size: "25",
  });

  return requestJson(tenantPath(tenantId, `/search?${parameters.toString()}`), {
    method: "GET",
    schema: searchResponseSchema,
    signal,
  });
}

export const projectionQueryKeys = {
  root: (tenantId: string) => ["projection", tenantId] as const,

  dashboard: (tenantId: string) => ["projection", tenantId, "dashboard"] as const,

  agents: (tenantId: string, pageSize: number) =>
    ["projection", tenantId, "agents", pageSize] as const,

  activity: (tenantId: string, pageSize: number) =>
    ["projection", tenantId, "activity", pageSize] as const,

  search: (tenantId: string, query: string) => ["projection", tenantId, "search", query] as const,
};

export function dashboardQueryOptions(tenantId: string) {
  return queryOptions({
    queryKey: projectionQueryKeys.dashboard(tenantId),
    queryFn: ({ signal }) => getDashboard(tenantId, signal),
    enabled: tenantId !== "",
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
  });
}

export function agentsQueryOptions(tenantId: string, pageSize = 10) {
  return queryOptions({
    queryKey: projectionQueryKeys.agents(tenantId, pageSize),
    queryFn: ({ signal }) => listAgentOverviews(tenantId, pageSize, signal),
    enabled: tenantId !== "",
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
  });
}

export function activityQueryOptions(tenantId: string, pageSize = 10) {
  return queryOptions({
    queryKey: projectionQueryKeys.activity(tenantId, pageSize),
    queryFn: ({ signal }) => listActivity(tenantId, pageSize, signal),
    enabled: tenantId !== "",
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
  });
}

export function searchQueryOptions(tenantId: string, query: string) {
  return queryOptions({
    queryKey: projectionQueryKeys.search(tenantId, query),
    queryFn: ({ signal }) => searchProjection(tenantId, query, signal),
    enabled: tenantId !== "" && query.trim().length >= 2,
    staleTime: 15_000,
  });
}
