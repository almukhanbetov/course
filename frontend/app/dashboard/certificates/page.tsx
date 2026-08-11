import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken, getCurrentUser } from "@/lib/session";
import { getMyCertificates } from "@/lib/api";

export const metadata: Metadata = {
  title: "Мои сертификаты — LMS Platform",
};

export default async function MyCertificatesPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const user = await getCurrentUser();

  let certificates: Awaited<ReturnType<typeof getMyCertificates>> = [];
  let error: string | null = null;

  try {
    certificates = await getMyCertificates(token);
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
  }

  return (
    <main>
      <h1>Мои сертификаты</h1>

      {error && <p role="alert">Не удалось загрузить сертификаты: {error}</p>}

      {!error && certificates.length === 0 && (
        <p>
          У вас пока нет сертификатов. Завершите курс, чтобы получить сертификат.{" "}
          <Link href="/dashboard/courses">Мои курсы</Link>
        </p>
      )}

      <div className="course-grid">
        {certificates.map((cert) => (
          <div key={cert.id} className="my-course-card">
            <h2>{cert.course_title}</h2>
            {user && <p className="my-course-meta">{user.first_name} {user.last_name}</p>}
            <p className="my-course-meta">{cert.certificate_number}</p>
            <p className="my-course-meta">
              Выдан: {new Date(cert.issued_at).toLocaleDateString("ru-RU")}
            </p>
            <Link href={`/dashboard/certificates/${cert.id}`} className="btn-primary">
              Открыть сертификат
            </Link>
          </div>
        ))}
      </div>
    </main>
  );
}
