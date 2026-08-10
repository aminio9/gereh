import type { SearchResult } from "./schemas";

const numberFormatter = new Intl.NumberFormat("fa-IR");

const dateFormatter = new Intl.DateTimeFormat("fa-IR", {
  dateStyle: "medium",
  timeStyle: "short",
});

const relativeFormatter = new Intl.RelativeTimeFormat("fa-IR", {
  numeric: "auto",
});

export function formatNumber(value: number): string {
  return numberFormatter.format(value);
}

export function formatDate(value?: string): string {
  if (!value) {
    return "—";
  }

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return "—";
  }

  return dateFormatter.format(date);
}

export function formatRelativeTime(value?: string): string {
  if (!value) {
    return "نامشخص";
  }

  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return "نامشخص";
  }

  const seconds = Math.round((date.getTime() - Date.now()) / 1000);
  const absoluteSeconds = Math.abs(seconds);

  if (absoluteSeconds < 60) {
    return relativeFormatter.format(seconds, "second");
  }

  const minutes = Math.round(seconds / 60);

  if (Math.abs(minutes) < 60) {
    return relativeFormatter.format(minutes, "minute");
  }

  const hours = Math.round(minutes / 60);

  if (Math.abs(hours) < 24) {
    return relativeFormatter.format(hours, "hour");
  }

  const days = Math.round(hours / 24);

  return relativeFormatter.format(days, "day");
}

const statusLabels: Record<string, string> = {
  active: "اکتیو",
  archived: "آرشیو",
  draft: "درفت",
  provisioning: "پروویژنینگ",
  configuring_runtime: "کانفیگ ران‌تایم",
  health_checking: "هلث‌چک",
  ready: "ردی",
  degraded: "دیگرید",
  paused: "پاز",
  failed: "فیلد",
  deleting: "دلیتینگ",
  deleted: "دلیت‌شده",

  backlog: "بک‌لاگ",
  in_progress: "این‌پراگرس",
  waiting_approval: "ویتینگ اپرووال",
  completed: "کامپلیت",
  canceled: "کنسل",

  planned: "پلند",
  on_hold: "آن‌هولد",
};

export function statusLabel(status: string): string {
  return statusLabels[status] ?? status;
}

export function searchTypeLabel(type: SearchResult["type"]): string {
  switch (type) {
    case "SEARCH_DOCUMENT_TYPE_COMPANY":
      return "کمپانی";

    case "SEARCH_DOCUMENT_TYPE_AGENT":
      return "ایجنت";

    case "SEARCH_DOCUMENT_TYPE_GOAL":
      return "گول";

    case "SEARCH_DOCUMENT_TYPE_PROJECT":
      return "پروجکت";

    case "SEARCH_DOCUMENT_TYPE_TASK":
      return "تسک";

    default:
      return "آیتم";
  }
}

export function activityLabel(eventType: string, fallback: string): string {
  switch (eventType) {
    case "task.created":
      return "تسک ساخته شد";

    case "task.updated":
      return "تسک آپدیت شد";

    case "task.status_changed":
      return "استیتس تسک تغییر کرد";

    case "task.assigned":
      return "تسک اساین شد";

    case "task.unassigned":
      return "اساینمنت تسک حذف شد";

    case "task.dependency_added":
      return "دیپندنسی به تسک اضافه شد";

    case "task.dependency_removed":
      return "دیپندنسی تسک حذف شد";

    case "task.comment_added":
      return "کامنت به تسک اضافه شد";

    case "task.comment_updated":
      return "کامنت تسک آپدیت شد";

    case "task.comment_deleted":
      return "کامنت تسک حذف شد";

    case "task.artifact_added":
      return "آرتیفکت به تسک اضافه شد";

    case "task.checklist_changed":
      return "چک‌لیست تسک تغییر کرد";

    case "task.schedule_changed":
      return "اسکجول تسک تغییر کرد";

    default:
      return fallback || eventType;
  }
}
