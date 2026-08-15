import type { Metadata } from "next";
import { getSpecialities } from "@/lib/api";
import { SpecialityCard } from "@/components/SpecialityCard";
import { PublicShell } from "@/components/shell/PublicShell";
import { ErrorState } from "@/components/shell/ErrorState";
import { IconGraduationCap } from "@/components/shell/icons";

export const metadata: Metadata = {
  title: "Специальности — LMS Platform",
};

export default async function SpecialitiesPage() {
  let specialities: Awaited<ReturnType<typeof getSpecialities>> = [];
  let error: string | null = null;

  try {
    specialities = await getSpecialities();
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
  }

  return (
    <PublicShell>
      <h1>Специальности</h1>
      <p className="subtitle">Пошаговые траектории обучения из нескольких курсов.</p>

      {error && <ErrorState message={`Не удалось загрузить специальности: ${error}`} />}

      {!error && specialities.length === 0 && (
        <div className="empty-state">
          <IconGraduationCap size={26} />
          <p className="empty-state-title">Специальностей пока нет</p>
          <p className="empty-state-text">Загляните позже — новые roadmap-траектории появятся здесь.</p>
        </div>
      )}

      {specialities.length > 0 && (
        <div className="speciality-grid">
          {specialities.map((speciality) => (
            <SpecialityCard key={speciality.id} speciality={speciality} />
          ))}
        </div>
      )}
    </PublicShell>
  );
}
