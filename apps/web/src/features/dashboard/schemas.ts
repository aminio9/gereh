import { z } from "zod";

const countSchema = z.coerce.number().int().nonnegative().optional().default(0);

export const projectionMetadataSchema = z.object({
  projectedThroughEventTime: z.string().optional(),
  lastProcessedAt: z.string().optional(),
});

export const dashboardSummarySchema = z.object({
  companiesTotal: countSchema,
  companiesActive: countSchema,

  agentsTotal: countSchema,
  agentsReady: countSchema,
  agentsDegraded: countSchema,
  agentsPaused: countSchema,
  agentsFailed: countSchema,

  goalsActive: countSchema,
  goalsCompleted: countSchema,

  projectsActive: countSchema,
  projectsOnHold: countSchema,
  projectsCompleted: countSchema,

  tasksTotal: countSchema,
  tasksBacklog: countSchema,
  tasksReady: countSchema,
  tasksInProgress: countSchema,
  tasksWaitingApproval: countSchema,
  tasksCompleted: countSchema,
  tasksCanceled: countSchema,
  tasksBlocked: countSchema,
});

export const dashboardResponseSchema = z.object({
  summary: dashboardSummarySchema.optional().default(() => dashboardSummarySchema.parse({})),
  metadata: projectionMetadataSchema.optional().default({}),
});

export const agentOverviewSchema = z.object({
  tenantId: z.string(),
  companyId: z.string(),
  agentId: z.string(),

  slug: z.string().default(""),
  displayName: z.string().default(""),
  roleTitle: z.string().default(""),
  status: z.string().default("unknown"),

  managerAgentId: z.string().optional(),

  assignedTaskCount: countSchema,
  activeTaskCount: countSchema,

  updatedAt: z.string().optional(),
});

export const agentOverviewResponseSchema = z.object({
  agents: z.array(agentOverviewSchema).optional().default([]),
  nextPageToken: z.string().optional().default(""),
  metadata: projectionMetadataSchema.optional().default({}),
});

export const activityItemSchema = z.object({
  eventId: z.string(),
  eventType: z.string(),

  companyId: z.string().optional(),
  projectId: z.string().optional(),
  taskId: z.string().optional(),

  actorType: z.string().optional(),
  actorId: z.string().optional(),

  summary: z.string().default(""),
  occurredAt: z.string().optional(),
});

export const activityResponseSchema = z.object({
  items: z.array(activityItemSchema).optional().default([]),
  nextPageToken: z.string().optional().default(""),
  metadata: projectionMetadataSchema.optional().default({}),
});

export const searchDocumentTypeSchema = z.enum([
  "SEARCH_DOCUMENT_TYPE_UNSPECIFIED",
  "SEARCH_DOCUMENT_TYPE_COMPANY",
  "SEARCH_DOCUMENT_TYPE_AGENT",
  "SEARCH_DOCUMENT_TYPE_GOAL",
  "SEARCH_DOCUMENT_TYPE_PROJECT",
  "SEARCH_DOCUMENT_TYPE_TASK",
]);

export const searchResultSchema = z.object({
  type: searchDocumentTypeSchema,
  id: z.string(),
  companyId: z.string().optional(),
  title: z.string().default(""),
  subtitle: z.string().default(""),
  status: z.string().default(""),
  rank: z.coerce.number().optional().default(0),
  updatedAt: z.string().optional(),
});

export const searchResponseSchema = z.object({
  results: z.array(searchResultSchema).optional().default([]),
  nextPageToken: z.string().optional().default(""),
  metadata: projectionMetadataSchema.optional().default({}),
});

export type DashboardResponse = z.infer<typeof dashboardResponseSchema>;
export type DashboardSummary = z.infer<typeof dashboardSummarySchema>;
export type AgentOverview = z.infer<typeof agentOverviewSchema>;
export type AgentOverviewResponse = z.infer<typeof agentOverviewResponseSchema>;
export type ActivityItem = z.infer<typeof activityItemSchema>;
export type ActivityResponse = z.infer<typeof activityResponseSchema>;
export type SearchResult = z.infer<typeof searchResultSchema>;
export type SearchResponse = z.infer<typeof searchResponseSchema>;
export type ProjectionMetadata = z.infer<typeof projectionMetadataSchema>;
