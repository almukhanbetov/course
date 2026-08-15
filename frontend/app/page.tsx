import Link from "next/link";
import { BackendStatus } from "@/components/BackendStatus";
import { IconAward, IconCourses, IconSparkles } from "@/components/shell/icons";
import { PublicShell } from "@/components/shell/PublicShell";

export default function HomePage() {
  return (
    <PublicShell>
      <div className="hero">
        <h1>Учитесь. Растите. Достигайте.</h1>
        <p className="subtitle">
          Специальности → Курсы → Модули → Уроки. Современная платформа для профессионального образования
          с персональными рекомендациями и сертификатами.
        </p>
        <div className="hero-actions">
          <Link href="/courses" className="btn-primary">
            Смотреть курсы →
          </Link>
          <Link href="/specialities" className="btn-secondary">
            Специальности
          </Link>
        </div>
        <div className="hero-status">
          <BackendStatus />
        </div>
      </div>

      <div className="value-grid">
        <div className="value-card">
          <div className="value-icon">
            <IconCourses size={20} />
          </div>
          <h3>Структурированные курсы</h3>
          <p>Модули, уроки, практика и итоговые тесты — понятный путь от новичка до профи.</p>
        </div>
        <div className="value-card">
          <div className="value-icon">
            <IconSparkles size={20} />
          </div>
          <h3>Персональные рекомендации</h3>
          <p>Подбираем курсы под ваш прогресс, интересы и выбранную специальность.</p>
        </div>
        <div className="value-card">
          <div className="value-icon">
            <IconAward size={20} />
          </div>
          <h3>Сертификаты</h3>
          <p>Подтверждайте пройденные курсы официальным сертификатом с уникальным номером.</p>
        </div>
      </div>
    </PublicShell>
  );
}
