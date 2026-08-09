import { Bot, Building2, FileSearch, FolderKanban, ListTodo, Search, Target } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useSearchParams } from "react-router";

import { searchQueryOptions } from "../api/projection";
import { formatDate, searchTypeLabel } from "../formatters";
import type { SearchResult } from "../schemas";
import { useTenantWorkspace } from "../../tenants/hooks/useTenantWorkspace";

export function SearchPage() {
  const { tenantId } = useTenantWorkspace();

  const [searchParams, setSearchParams] = useSearchParams();

  const queryFromUrl = searchParams.get("q")?.trim() ?? "";

  const [value, setValue] = useState(queryFromUrl);

  const [previousQuery, setPreviousQuery] = useState(queryFromUrl);

  if (previousQuery !== queryFromUrl) {
    setPreviousQuery(queryFromUrl);
    setValue(queryFromUrl);
  }

  const query = useQuery(searchQueryOptions(tenantId, queryFromUrl));

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const next = value.trim();

    if (next.length < 2) {
      return;
    }

    setSearchParams({ q: next });
  };

  return (
    <div className="search-page">
      <header className="page-heading">
        <div>
          <p className="page-heading__eyebrow">GLOBAL SEARCH</p>

          <h2>سرچ</h2>

          <p>سرچ بین کمپانی‌ها، ایجنت‌ها، گول‌ها، پروجکت‌ها و تسک‌ها.</p>
        </div>
      </header>

      <form className="search-page__form" role="search" onSubmit={submit}>
        <Search size={19} />

        <input
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="حداقل دو کاراکتر وارد کنید…"
          aria-label="عبارت سرچ"
        />

        <button type="submit" className="primary-button primary-button--compact">
          سرچ
        </button>
      </form>

      {queryFromUrl.length < 2 ? (
        <div className="search-placeholder">
          <FileSearch size={28} />
          <p>عبارت سرچ را وارد کنید.</p>
        </div>
      ) : query.isPending ? (
        <div className="panel-empty">در حال سرچ…</div>
      ) : query.isError ? (
        <div className="inline-error">{query.error.message}</div>
      ) : query.data.results.length === 0 ? (
        <div className="search-placeholder">
          <FileSearch size={28} />
          <p>نتیجه‌ای پیدا نشد.</p>
        </div>
      ) : (
        <div className="search-results">
          {query.data.results.map((result) => (
            <article className="search-result" key={`${result.type}:${result.id}`}>
              <div className="search-result__icon">
                <SearchTypeIcon result={result} />
              </div>

              <div className="search-result__content">
                <div className="search-result__title">
                  <strong>{result.title}</strong>

                  <span>{searchTypeLabel(result.type)}</span>
                </div>

                {result.subtitle && <p>{result.subtitle}</p>}

                <span className="search-result__date">{formatDate(result.updatedAt)}</span>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}

function SearchTypeIcon({ result }: { result: SearchResult }) {
  switch (result.type) {
    case "SEARCH_DOCUMENT_TYPE_COMPANY":
      return <Building2 size={18} />;

    case "SEARCH_DOCUMENT_TYPE_AGENT":
      return <Bot size={18} />;

    case "SEARCH_DOCUMENT_TYPE_GOAL":
      return <Target size={18} />;

    case "SEARCH_DOCUMENT_TYPE_PROJECT":
      return <FolderKanban size={18} />;

    case "SEARCH_DOCUMENT_TYPE_TASK":
      return <ListTodo size={18} />;

    default:
      return <FileSearch size={18} />;
  }
}
