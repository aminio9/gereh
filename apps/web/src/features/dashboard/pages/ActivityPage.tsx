import { Activity } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { activityQueryOptions } from "../api/projection";
import { activityLabel, formatDate } from "../formatters";
import { useTenantWorkspace } from "../../tenants/hooks/useTenantWorkspace";

export function ActivityPage() {
  const { tenantId } = useTenantWorkspace();

  const query = useQuery(activityQueryOptions(tenantId, 25));

  return (
    <div className="resource-page">
      <header className="page-heading">
        <div>
          <p className="page-heading__eyebrow">ACTIVITY FEED</p>

          <h2>اکتیویتی</h2>

          <p>تغییرات اخیر تسک‌ها و ورک‌فلوهای سازمان.</p>
        </div>
      </header>

      <section className="panel">
        {query.isPending ? (
          <div className="panel-empty">در حال لود اکتیویتی…</div>
        ) : query.isError ? (
          <div className="inline-error">{query.error.message}</div>
        ) : query.data.items.length === 0 ? (
          <div className="panel-empty">هنوز اکتیویتی وجود ندارد.</div>
        ) : (
          <div className="timeline">
            {query.data.items.map((item) => (
              <article className="timeline-item" key={item.eventId}>
                <div className="timeline-item__icon">
                  <Activity size={15} />
                </div>

                <div className="timeline-item__content">
                  <strong>{activityLabel(item.eventType, item.summary)}</strong>

                  <div className="timeline-item__meta">
                    <span dir="ltr">{item.eventType}</span>

                    <span>{formatDate(item.occurredAt)}</span>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
