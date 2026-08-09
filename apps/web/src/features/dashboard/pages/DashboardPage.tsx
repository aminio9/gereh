import { Bot, CircleAlert, FolderKanban, ListTodo, Network } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { activityQueryOptions, agentsQueryOptions, dashboardQueryOptions } from "../api/projection";
import { MetricCard } from "../components/MetricCard";
import { StatusBadge } from "../components/StatusBadge";
import { StatusMeter } from "../components/StatusMeter";
import { activityLabel, formatNumber, formatRelativeTime } from "../formatters";
import { useTenantWorkspace } from "../../tenants/hooks/useTenantWorkspace";

export function DashboardPage() {
  const { tenantId } = useTenantWorkspace();

  const dashboardQuery = useQuery(dashboardQueryOptions(tenantId));

  const agentsQuery = useQuery(agentsQueryOptions(tenantId, 6));

  const activityQuery = useQuery(activityQueryOptions(tenantId, 8));

  if (dashboardQuery.isPending) {
    return <DashboardSkeleton />;
  }

  if (dashboardQuery.isError) {
    return (
      <section className="error-state">
        <CircleAlert size={22} />

        <div>
          <h2>داشبورد لود نشد</h2>
          <p>{dashboardQuery.error.message}</p>
        </div>
      </section>
    );
  }

  const summary = dashboardQuery.data.summary;
  const activeTasks = summary.tasksReady + summary.tasksInProgress + summary.tasksWaitingApproval;

  const freshness =
    dashboardQuery.data.metadata.lastProcessedAt ??
    dashboardQuery.data.metadata.projectedThroughEventTime;

  return (
    <div className="dashboard-page">
      <section className="page-heading">
        <div>
          <p className="page-heading__eyebrow">اپریشنال اورویو</p>

          <h2>داشبورد تننت</h2>

          <p>وضعیت فعلی کمپانی، ایجنت‌ها، پروجکت‌ها و تسک‌ها از پروجکشن خوانده می‌شود.</p>
        </div>

        <div className="freshness-pill">
          <span className="freshness-pill__dot" />
          پروجکشن {formatRelativeTime(freshness)}
        </div>
      </section>

      <section className="metric-grid" aria-label="متریک‌های اصلی">
        <MetricCard
          icon={Bot}
          label="ایجنت‌ها"
          value={summary.agentsTotal}
          detail={`${formatNumber(summary.agentsReady)} ردی`}
          tone={summary.agentsFailed > 0 ? "danger" : "good"}
        />

        <MetricCard
          icon={ListTodo}
          label="تسک‌های اکتیو"
          value={activeTasks}
          detail={`${formatNumber(summary.tasksTotal)} تسک در مجموع`}
        />

        <MetricCard
          icon={CircleAlert}
          label="تسک‌های بلاک"
          value={summary.tasksBlocked}
          detail="نیازمند بررسی دیپندنسی"
          tone={summary.tasksBlocked > 0 ? "danger" : "good"}
        />

        <MetricCard
          icon={FolderKanban}
          label="پروجکت‌های اکتیو"
          value={summary.projectsActive}
          detail={`${formatNumber(summary.projectsOnHold)} آن‌هولد`}
        />

        <MetricCard
          icon={Network}
          label="کمپانی‌ها"
          value={summary.companiesTotal}
          detail={`${formatNumber(summary.companiesActive)} اکتیو`}
        />
      </section>

      {summary.agentsTotal === 0 && (
        <section className="attention-banner">
          <Bot size={18} />

          <div>
            <strong>هنوز ایجنتی وجود ندارد</strong>
            <p>بعد از ساخت اولین ایجنت، وضعیت آن در این داشبورد ظاهر می‌شود.</p>
          </div>
        </section>
      )}

      <div className="dashboard-columns">
        <section className="panel">
          <header className="panel-header">
            <div>
              <p className="panel-eyebrow">AGENTS</p>
              <h3>استیتس ایجنت‌ها</h3>
            </div>

            <span className="panel-total">{formatNumber(summary.agentsTotal)}</span>
          </header>

          <div className="status-meter-list">
            <StatusMeter
              label="ردی"
              value={summary.agentsReady}
              total={summary.agentsTotal}
              tone="good"
            />

            <StatusMeter
              label="دیگرید"
              value={summary.agentsDegraded}
              total={summary.agentsTotal}
              tone="warning"
            />

            <StatusMeter
              label="پاز"
              value={summary.agentsPaused}
              total={summary.agentsTotal}
              tone="neutral"
            />

            <StatusMeter
              label="فیلد"
              value={summary.agentsFailed}
              total={summary.agentsTotal}
              tone="danger"
            />
          </div>
        </section>

        <section className="panel">
          <header className="panel-header">
            <div>
              <p className="panel-eyebrow">TASKS</p>
              <h3>استیتس تسک‌ها</h3>
            </div>

            <span className="panel-total">{formatNumber(summary.tasksTotal)}</span>
          </header>

          <div className="status-meter-list">
            <StatusMeter
              label="بک‌لاگ"
              value={summary.tasksBacklog}
              total={summary.tasksTotal}
              tone="neutral"
            />

            <StatusMeter
              label="ردی"
              value={summary.tasksReady}
              total={summary.tasksTotal}
              tone="good"
            />

            <StatusMeter
              label="این‌پراگرس"
              value={summary.tasksInProgress}
              total={summary.tasksTotal}
              tone="good"
            />

            <StatusMeter
              label="ویتینگ اپرووال"
              value={summary.tasksWaitingApproval}
              total={summary.tasksTotal}
              tone="warning"
            />
          </div>
        </section>
      </div>

      <div className="dashboard-columns">
        <section className="panel">
          <header className="panel-header">
            <div>
              <p className="panel-eyebrow">AGENTS</p>
              <h3>ایجنت‌های اخیر</h3>
            </div>
          </header>

          {agentsQuery.isError ? (
            <p className="inline-error">ایجنت‌ها لود نشدند.</p>
          ) : agentsQuery.data?.agents.length ? (
            <div className="agent-list">
              {agentsQuery.data.agents.map((agent) => (
                <article className="agent-row" key={agent.agentId}>
                  <div className="agent-avatar">
                    <Bot size={16} />
                  </div>

                  <div className="agent-row__identity">
                    <strong>{agent.displayName || agent.slug}</strong>

                    <span>{agent.roleTitle || "بدون رول"}</span>
                  </div>

                  <div className="agent-row__work">
                    <strong>{formatNumber(agent.activeTaskCount)}</strong>

                    <span>تسک اکتیو</span>
                  </div>

                  <StatusBadge status={agent.status} />
                </article>
              ))}
            </div>
          ) : (
            <div className="panel-empty">ایجنتی برای نمایش وجود ندارد.</div>
          )}
        </section>

        <section className="panel">
          <header className="panel-header">
            <div>
              <p className="panel-eyebrow">ACTIVITY</p>
              <h3>اکتیویتی اخیر</h3>
            </div>
          </header>

          {activityQuery.isError ? (
            <p className="inline-error">اکتیویتی لود نشد.</p>
          ) : activityQuery.data?.items.length ? (
            <div className="activity-list">
              {activityQuery.data.items.map((item) => (
                <article className="activity-row" key={item.eventId}>
                  <div className="activity-row__marker" aria-hidden="true" />

                  <div className="activity-row__body">
                    <strong>{activityLabel(item.eventType, item.summary)}</strong>

                    <span>{formatRelativeTime(item.occurredAt)}</span>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <div className="panel-empty">هنوز اکتیویتی ثبت نشده است.</div>
          )}
        </section>
      </div>
    </div>
  );
}

function DashboardSkeleton() {
  return (
    <div className="dashboard-skeleton" aria-label="در حال لود داشبورد">
      <div className="skeleton skeleton--heading" />

      <div className="metric-grid">
        {Array.from({ length: 5 }, (_, index) => (
          <div key={index} className="skeleton skeleton--metric" />
        ))}
      </div>

      <div className="dashboard-columns">
        <div className="skeleton skeleton--panel" />
        <div className="skeleton skeleton--panel" />
      </div>
    </div>
  );
}
