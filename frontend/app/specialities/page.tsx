import type { Metadata } from "next";
import { getSpecialities } from "@/lib/api";
import { SpecialityCard } from "@/components/SpecialityCard";
import { PublicShell } from "@/components/shell/PublicShell";
import { ErrorState } from "@/components/shell/ErrorState";
import { IconGraduationCap } from "@/components/shell/icons";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { getLocale } from "@/lib/i18n/getLocale";

export const metadata: Metadata = {
  title: "Специальности — LMS Platform",
};

export default async function SpecialitiesPage() {
  const locale = await getLocale();
  const dict = getDictionary(locale).specialities;
  let specialities: Awaited<ReturnType<typeof getSpecialities>> = [];
  let error: string | null = null;

  try {
    specialities = await getSpecialities();
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
  }

  return (
    <PublicShell locale={locale}>
      <h1>{dict.title}</h1>
      <p className="subtitle">{dict.subtitle}</p>

      {error && <ErrorState message={dict.errorLoad(error)} />}

      {!error && specialities.length === 0 && (
        <div className="empty-state">
          <IconGraduationCap size={26} />
          <p className="empty-state-title">{dict.emptyTitle}</p>
          <p className="empty-state-text">{dict.emptyText}</p>
        </div>
      )}

      {specialities.length > 0 && (
        <div className="speciality-grid">
          {specialities.map((speciality) => (
            <SpecialityCard key={speciality.id} speciality={speciality} locale={locale} />
          ))}
        </div>
      )}
    </PublicShell>
  );
}
