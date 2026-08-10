import { Network } from "lucide-react";

import { useAuth } from "../context/AuthContext";

export function LoginPage() {
  const { login } = useAuth();

  return (
    <main className="login-page">
      <section className="login-card">
        <div className="brand-mark brand-mark--large" aria-hidden="true">
          <Network size={26} />
        </div>

        <p className="login-card__eyebrow">GEREH CONTROL PLANE</p>

        <h1>گِره</h1>

        <p className="login-card__lead">پلتفرم هماهنگی کمپانی‌ها، ایجنت‌ها و تسک‌های هوش مصنوعی.</p>

        <button className="primary-button" type="button" onClick={() => login("/")}>
          ورود به گِره
        </button>
      </section>
    </main>
  );
}
