import Link from "next/link";
import type { Speciality } from "@/lib/api";
import { IconGraduationCap } from "@/components/shell/icons";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { localizeDescription, localizeTitle } from "@/lib/i18n/localize";
import type { Locale } from "@/lib/i18n/locale";

export function SpecialityCard({ speciality, locale }: { speciality: Speciality; locale: Locale }) {
  return (
    <Link href={`/specialities/${speciality.id}`} className="speciality-card">
      <div className="speciality-card-icon">
        {speciality.image_url ? <img src={speciality.image_url} alt="" /> : <IconGraduationCap size={22} />}
      </div>
      <h2>{localizeTitle(speciality, locale)}</h2>
      <p className="course-card-description">{localizeDescription(speciality, locale)}</p>
      <span className="speciality-card-cta">{getDictionary(locale).specialities.cardCta}</span>
    </Link>
  );
}
