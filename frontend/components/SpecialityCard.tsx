import Link from "next/link";
import type { Speciality } from "@/lib/api";

export function SpecialityCard({ speciality }: { speciality: Speciality }) {
  return (
    <Link href={`/specialities/${speciality.id}`} className="course-card">
      <h2>{speciality.title}</h2>
      <p className="course-card-description">{speciality.description}</p>
    </Link>
  );
}
