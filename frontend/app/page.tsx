import Link from "next/link";
import { BackendStatus } from "@/components/BackendStatus";

export default function HomePage() {
  return (
    <main>
      <h1>LMS Platform</h1>
      <p className="subtitle">
        Специальности → Курсы → Модули → Уроки. Ранняя стадия проекта.
      </p>
      <div className="status">
        <BackendStatus />
      </div>
      <Link href="/courses" className="nav-link">
        Смотреть курсы →
      </Link>
    </main>
  );
}
