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
import { getDictionary } from "@/lib/i18n/dictionaries";
import { getLocale } from "@/lib/i18n/getLocale";

export default async function HomePage() {
  const locale = await getLocale();
  const dict = getDictionary(locale).home;
  const [popularCourses, specialities] = await Promise.all([
    getCourses({ sort: "rating", limit: 6 }).catch(() => null),
    getSpecialities().catch(() => []),
  ]);

  return (
    <PublicShell locale={locale}>
      <div className="hero">
        <h1>{dict.heroTitle}</h1>
        <p className="subtitle">{dict.heroSubtitle}</p>
        <div className="hero-actions">
          <Link href="/courses" className="btn-primary">
            {dict.heroCtaCourses}
          </Link>
          <Link href="/specialities" className="btn-secondary">
            {dict.heroCtaSpecialities}
          </Link>
        </div>
        <div className="hero-status">
          <BackendStatus locale={locale} />
        </div>
      </div>

      <section className="home-section">
        <p className="section-eyebrow">{dict.eyebrowCatalog}</p>
        <div className="section-header">
          <h2>{dict.popularCoursesTitle}</h2>
          <Link href="/courses" className="nav-link">
            {dict.allCoursesLink}
          </Link>
        </div>
        <p className="section-subtitle">{dict.popularCoursesSubtitle}</p>
        {popularCourses && popularCourses.items.length > 0 ? (
          <div className="course-grid">
            {popularCourses.items.map((course) => (
              <CourseCard key={course.id} course={course} locale={locale} />
            ))}
          </div>
        ) : (
          <p className="subtitle">{dict.noCoursesYet}</p>
        )}
      </section>

      <section className="home-section">
        <p className="section-eyebrow">{dict.specialitiesEyebrow}</p>
        <div className="section-header">
          <h2>{dict.specialitiesTitle}</h2>
          <Link href="/specialities" className="nav-link">
            {dict.allSpecialitiesLink}
          </Link>
        </div>
        <p className="section-subtitle">{dict.specialitiesSubtitle}</p>
        {specialities.length > 0 ? (
          <div className="speciality-grid">
            {specialities.map((s) => (
              <SpecialityCard key={s.id} speciality={s} locale={locale} />
            ))}
          </div>
        ) : (
          <p className="subtitle">{dict.noSpecialitiesYet}</p>
        )}
      </section>

      <section className="home-section">
        <p className="section-eyebrow">{dict.whyEyebrow}</p>
        <h2>{dict.whyTitle}</h2>
        <p className="section-subtitle">{dict.whySubtitle}</p>
        <div className="value-grid">
          <div className="value-card">
            <div className="value-icon">
              <IconLayers size={20} />
            </div>
            <h3>{dict.feature1Title}</h3>
            <p>{dict.feature1Text}</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconClipboard size={20} />
            </div>
            <h3>{dict.feature2Title}</h3>
            <p>{dict.feature2Text}</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconBarChart size={20} />
            </div>
            <h3>{dict.feature3Title}</h3>
            <p>{dict.feature3Text}</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconAward size={20} />
            </div>
            <h3>{dict.feature4Title}</h3>
            <p>{dict.feature4Text}</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconGraduationCap size={20} />
            </div>
            <h3>{dict.feature5Title}</h3>
            <p>{dict.feature5Text}</p>
          </div>
          <div className="value-card">
            <div className="value-icon">
              <IconUsers size={20} />
            </div>
            <h3>{dict.feature6Title}</h3>
            <p>{dict.feature6Text}</p>
          </div>
        </div>
      </section>

      <section className="home-cta">
        <IconCourses size={26} />
        <h2>{dict.ctaTitle}</h2>
        <p className="subtitle">{dict.ctaSubtitle}</p>
        <Link href="/register" className="btn-primary">
          {dict.ctaButton}
        </Link>
      </section>
    </PublicShell>
  );
}
