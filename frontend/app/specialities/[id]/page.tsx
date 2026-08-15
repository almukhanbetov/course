import Link from "next/link";
import { notFound } from "next/navigation";
import { getSpeciality, getMyRoadmap } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { pluralizeRu } from "@/lib/pluralize";
import { ErrorState } from "@/components/shell/ErrorState";
import { IconGraduationCap } from "@/components/shell/icons";

export default async function SpecialityDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let speciality: Awaited<ReturnType<typeof getSpeciality>>;
  let error: string | null = null;

  try {
    speciality = await getSpeciality(id);
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
    speciality = null;
  }

  if (error) {
    return (
      <main>
        <ErrorState message={`Не удалось загрузить специальность: ${error}`} />
      </main>
    );
  }

  if (!speciality) {
    notFound();
  }

  const token = await getSessionToken();
  const roadmap = token ? await getMyRoadmap(token, id) : null;

  const progressByCourseId = new Map((roadmap?.courses ?? []).map((c) => [c.course_id, c]));

  return (
    <main className="speciality-detail-shell">
      <nav className="breadcrumbs" aria-label="Хлебные крошки">
        <Link href="/">Главная</Link>
        <span>/</span>
        <Link href="/specialities">Специальности</Link>
        <span>/</span>
        <span>{speciality.title}</span>
      </nav>

      <div className="course-hero">
        <div className="speciality-detail-icon">
          <IconGraduationCap size={22} />
        </div>
        <h1>{speciality.title}</h1>
        <p className="course-hero-description">{speciality.description}</p>
        <p className="subtitle">
          {speciality.courses.length} {pluralizeRu(speciality.courses.length, "курс", "курса", "курсов")} в roadmap
        </p>

        {roadmap && (
          <div className="status">
            <p>
              <strong>Общий прогресс:</strong> {roadmap.progress_percent}%
              {roadmap.completed ? " · специальность завершена" : ""}
            </p>
            <div className="progress-bar">
              <div className="progress-bar-fill" style={{ width: `${roadmap.progress_percent}%` }} />
            </div>
          </div>
        )}

        {!token && (
          <p className="subtitle">
            <Link href="/login">Войдите</Link>, чтобы видеть свой прогресс по этой специальности.
          </p>
        )}
      </div>

      <h2>Roadmap</h2>
      <ol className="roadmap">
        {speciality.courses.map((course) => {
          const progress = progressByCourseId.get(course.id);
          return (
            <li key={course.id} className="roadmap-step">
              <span className={`roadmap-marker${progress?.completed ? " completed" : ""}`}>
                {progress?.completed ? "✓" : course.position}
              </span>
              <div className="roadmap-card">
                <h3>{course.title}</h3>
                <p className="course-card-description">{course.description}</p>
                <div>
                  <span className="badge">{course.level}</span>
                  {!course.required && <span className="badge">опционально</span>}
                </div>
                {progress && (
                  <>
                    <div className="progress-bar">
                      <div className="progress-bar-fill" style={{ width: `${progress.progress_percent}%` }} />
                    </div>
                    <p className="my-course-meta">
                      {progress.progress_percent}%{progress.completed ? " · завершён" : ""}
                    </p>
                  </>
                )}
                <Link href={`/courses/${course.id}`} className="nav-link">
                  Перейти к курсу →
                </Link>
              </div>
            </li>
          );
        })}
      </ol>
    </main>
  );
}
