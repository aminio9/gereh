import { isRouteErrorResponse, Link, useRouteError } from "react-router";

export function RouteErrorPage() {
  const error = useRouteError();

  let message = "An unexpected application error occurred.";

  if (isRouteErrorResponse(error)) {
    message = `${error.status} ${error.statusText}`;
  } else if (error instanceof Error) {
    message = error.message;
  }

  return (
    <main className="page-shell">
      <section className="hero-card">
        <p className="eyebrow">Application error</p>
        <h1>Gereh could not load this page.</h1>
        <p role="alert">{message}</p>
        <Link to="/">Return home</Link>
      </section>
    </main>
  );
}
