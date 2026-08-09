import { Bot } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { agentsQueryOptions } from "../api/projection";
import { StatusBadge } from "../components/StatusBadge";
import { formatDate, formatNumber } from "../formatters";
import { useTenantWorkspace } from "../../tenants/hooks/useTenantWorkspace";

export function AgentsPage() {
  const { tenantId } = useTenantWorkspace();

  const query = useQuery(agentsQueryOptions(tenantId, 50));

  return (
    <div className="resource-page">
      <header className="page-heading">
        <div>
          <p className="page-heading__eyebrow">AGENT OVERVIEW</p>

          <h2>ایجنت‌ها</h2>

          <p>اورویوی ایجنت‌ها از پروجکشن سرویس.</p>
        </div>
      </header>

      <section className="panel">
        {query.isPending ? (
          <div className="panel-empty">در حال لود ایجنت‌ها…</div>
        ) : query.isError ? (
          <div className="inline-error">{query.error.message}</div>
        ) : query.data.agents.length === 0 ? (
          <div className="panel-empty">هیچ ایجنتی وجود ندارد.</div>
        ) : (
          <div className="data-table-wrapper">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ایجنت</th>
                  <th>رول</th>
                  <th>استیتس</th>
                  <th>تسک اکتیو</th>
                  <th>کل اساینمنت</th>
                  <th>آخرین آپدیت</th>
                </tr>
              </thead>

              <tbody>
                {query.data.agents.map((agent) => (
                  <tr key={agent.agentId}>
                    <td>
                      <div className="table-entity">
                        <div className="agent-avatar">
                          <Bot size={15} />
                        </div>

                        <div>
                          <strong>{agent.displayName || agent.slug}</strong>

                          <span dir="ltr">{agent.slug}</span>
                        </div>
                      </div>
                    </td>

                    <td>{agent.roleTitle || "—"}</td>

                    <td>
                      <StatusBadge status={agent.status} />
                    </td>

                    <td>{formatNumber(agent.activeTaskCount)}</td>

                    <td>{formatNumber(agent.assignedTaskCount)}</td>

                    <td>{formatDate(agent.updatedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
