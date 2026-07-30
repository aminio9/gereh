import { Link } from "react-router";

export function NotFoundPage() {
  return (
    <main className="page-shell">
      <section className="hero-card">
        <p className="eyebrow">404</p>
        <h1>Page not found</h1>
        <p>The requested Gereh page does not exist.</p>
        <Link to="/">Return to the dashboard</Link>
      </section>
    </main>
  );
}
