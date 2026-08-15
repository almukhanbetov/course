import Link from "next/link";
import { BackendStatus } from "@/components/BackendStatus";
import {
  IconAward,
  IconBarChart,
  IconClipboard,
  IconCourses,
  IconGraduationCap,
  IconLayers,
  IconUsers,
} from "@/components/shell/icons";
import { PublicShell } from "@/components/shell/PublicShell";
import { CourseCard } from "@/components/CourseCard";
import { SpecialityCard } from "@/components/SpecialityCard";
import { getCourses, getSpecialities } from "@/lib/api";

export default async function HomePage() {
  const [popularCourses, specialities] = await Promise.all([
    getCourses({ sort: "rating", limit: 6 }).catch(() => null),
    getSpecialities().catch(() => []),
  ]);

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

      <section className="home-section">
        <div className="section-header">
          <h2>Популярные курсы</h2>
          <Link href="/courses" className="nav-link">
            Все курсы →
          </Link>
        </div>
        {popularCourses && popularCourses.items.length > 0 ? (
          <div className="course-grid">
            {popularCourses.items.map((course) => (
              <CourseCard key={course.id} course={course} />
            ))}
          </div>
        ) : (
          <p className="subtitle">Курсы скоро появятся здесь.</p>
        )}
      </section>

      <section className="home-section">
        <div className="section-header">
          <h2>Специальности</h2>
          <Link href="/specialities" className="nav-link">
            Все специальности →
          </Link>
        </div>
        {specialities.length > 0 ? (
          <div className="speciality-grid">
            {specialities.map((s) => (
              <SpecialityCard key={s.id} speciality={s} />
            ))}
          </div>
        ) : (
          <p className="subtitle">Специальности скоро появятся здесь.</p>
        )}
      </section>

      <section className="home-section">
        <h2>Почему выбирают нашу платформу</h2>
        <div className="value-grid">
          <div className="value-card">
            <div className="value-icon">
              <IconLayers size={20} />
            </div>
            <h3>Структурированные курсы</h3>
            <p>Модули, уроки, практика и итоговые тесты — понятный путь от новичка до профи.</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconClipboard size={20} />
            </div>
            <h3>Практические задания</h3>
            <p>Код-упражнения и домашние работы с проверкой — не только теория.</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconBarChart size={20} />
            </div>
            <h3>Отслеживание прогресса</h3>
            <p>Видите, сколько уроков пройдено и что осталось до завершения курса.</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconAward size={20} />
            </div>
            <h3>Сертификаты</h3>
            <p>Подтверждайте пройденные курсы официальным сертификатом с уникальным номером.</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconGraduationCap size={20} />
            </div>
            <h3>Roadmap по специальностям</h3>
            <p>Последовательный путь из нескольких курсов к конкретной IT-профессии.</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconUsers size={20} />
            </div>
            <h3>Опытные преподаватели</h3>
            <p>Курсы ведут практикующие специалисты, а не просто дикторы текста.</p>
          </div>
        </div>
      </section>

      <section className="home-cta">
        <IconCourses size={26} />
        <h2>Готовы начать учиться?</h2>
        <p className="subtitle">Выберите специальность или курс и сделайте первый шаг уже сегодня.</p>
        <Link href="/register" className="btn-primary">
          Начать бесплатно →
        </Link>
      </section>
    </PublicShell>
  );
}
