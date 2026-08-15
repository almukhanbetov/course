import Link from "next/link";
import type { Speciality } from "@/lib/api";
import { IconGraduationCap } from "@/components/shell/icons";

export function SpecialityCard({ speciality }: { speciality: Speciality }) {
  return (
    <Link href={`/specialities/${speciality.id}`} className="speciality-card">
      <div className="speciality-card-icon">
        {speciality.image_url ? <img src={speciality.image_url} alt="" /> : <IconGraduationCap size={22} />}
      </div>
      <h2>{speciality.title}</h2>
      <p className="course-card-description">{speciality.description}</p>
      <span className="speciality-card-cta">Смотреть roadmap →</span>
    </Link>
  );
}
