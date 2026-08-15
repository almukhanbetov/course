import type { Metadata } from "next";
import { getSpecialities } from "@/lib/api";
import { SpecialityCard } from "@/components/SpecialityCard";
import { PublicShell } from "@/components/shell/PublicShell";

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

      {error && <p role="alert">Не удалось загрузить специальности: {error}</p>}
      {!error && specialities.length === 0 && <p>Специальностей пока нет.</p>}

      <div className="speciality-grid">
        {specialities.map((speciality) => (
          <SpecialityCard key={speciality.id} speciality={speciality} />
        ))}
      </div>
    </PublicShell>
  );
}
