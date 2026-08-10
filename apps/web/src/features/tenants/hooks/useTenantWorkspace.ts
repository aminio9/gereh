import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";

import { tenantsQueryOptions } from "../api/listTenants";

export function useTenantWorkspace() {
  const { tenantId = "" } = useParams<{ tenantId: string }>();

  const tenantsQuery = useQuery(tenantsQueryOptions());

  const selectedContext = tenantsQuery.data?.contexts.find(
    (context) => context.tenant.tenantId === tenantId,
  );

  return {
    tenantId,
    tenantContexts: tenantsQuery.data?.contexts ?? [],
    selectedContext,
    selectedTenant: selectedContext?.tenant,
    isLoading: tenantsQuery.isLoading,
    error: tenantsQuery.error,
  };
}
