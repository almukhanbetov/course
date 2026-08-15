import Link from "next/link";
import { notFound } from "next/navigation";
import { getSpeciality, getMyRoadmap } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { ErrorState } from "@/components/shell/ErrorState";
import { IconGraduationCap } from "@/components/shell/icons";
import { getDictionary, localizeLevel } from "@/lib/i18n/dictionaries";
import { localizeDescription, localizeTitle } from "@/lib/i18n/localize";
import { getLocale } from "@/lib/i18n/getLocale";

export default async function SpecialityDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const locale = await getLocale();
  const dict = getDictionary(locale);

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
        <ErrorState message={dict.specialityDetail.errorLoad(error)} />
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
      <nav className="breadcrumbs" aria-label={dict.breadcrumbs.ariaLabel}>
        <Link href="/">{dict.breadcrumbs.home}</Link>
        <span>/</span>
        <Link href="/specialities">{dict.breadcrumbs.specialities}</Link>
        <span>/</span>
        <span>{localizeTitle(speciality, locale)}</span>
      </nav>

      <div className="course-hero">
        <div className="speciality-detail-icon">
          <IconGraduationCap size={22} />
        </div>
        <h1>{localizeTitle(speciality, locale)}</h1>
        <p className="course-hero-description">{localizeDescription(speciality, locale)}</p>
        <p className="subtitle">{dict.specialityDetail.coursesCount(speciality.courses.length)}</p>

        {roadmap && (
          <div className="status">
            <p>
              <strong>{dict.specialityDetail.progressLabel}</strong> {roadmap.progress_percent}%
              {roadmap.completed ? dict.specialityDetail.completedSuffix : ""}
            </p>
            <div className="progress-bar">
              <div className="progress-bar-fill" style={{ width: `${roadmap.progress_percent}%` }} />
            </div>
          </div>
        )}

        {!token && (
          <p className="subtitle">
            {dict.specialityDetail.loginHintPrefix}
            <Link href="/login">{dict.specialityDetail.loginHintLink}</Link>
            {dict.specialityDetail.loginHintSuffix}
          </p>
        )}
      </div>

      <h2>{dict.specialityDetail.roadmapTitle}</h2>
      <ol className="roadmap">
        {speciality.courses.map((course) => {
          const progress = progressByCourseId.get(course.id);
          return (
            <li key={course.id} className="roadmap-step">
              <span className={`roadmap-marker${progress?.completed ? " completed" : ""}`}>
                {progress?.completed ? "✓" : course.position}
              </span>
              <div className="roadmap-card">
                <h3>{localizeTitle(course, locale)}</h3>
                <p className="course-card-description">{localizeDescription(course, locale)}</p>
                <div>
                  <span className="badge">{localizeLevel(course.level, dict)}</span>
                  {!course.required && <span className="badge">{dict.specialityDetail.optionalBadge}</span>}
                </div>
                {progress && (
                  <>
                    <div className="progress-bar">
                      <div className="progress-bar-fill" style={{ width: `${progress.progress_percent}%` }} />
                    </div>
                    <p className="my-course-meta">
                      {progress.progress_percent}%
                      {progress.completed ? dict.specialityDetail.progressCompletedSuffix : ""}
                    </p>
                  </>
                )}
                <Link href={`/courses/${course.id}`} className="nav-link">
                  {dict.specialityDetail.goToCourseLink}
                </Link>
              </div>
            </li>
          );
        })}
      </ol>
    </main>
  );
}
