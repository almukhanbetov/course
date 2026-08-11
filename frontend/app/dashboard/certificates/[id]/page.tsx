import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { getMyCertificateDetail } from "@/lib/api";

export default async function CertificateDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const certificate = await getMyCertificateDetail(token, id);
  if (!certificate) {
    notFound();
  }

  const issuedDate = new Date(certificate.issued_at).toLocaleDateString("ru-RU", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });

  return (
    <>
      <Link href="/dashboard/certificates">← Мои сертификаты</Link>

      <div className="certificate-document">
        <p className="certificate-brand">LMS Platform</p>
        <p className="certificate-kind">Certificate of Completion</p>

        <p className="certificate-line">Настоящим подтверждается, что</p>
        <p className="certificate-name">{certificate.student_name}</p>
        <p className="certificate-line">успешно завершил(а) курс</p>
        <p className="certificate-course">{certificate.course_title}</p>

        <div className="certificate-footer">
          <div>
            <p className="certificate-label">Дата выдачи</p>
            <p>{issuedDate}</p>
          </div>
          <div>
            <p className="certificate-label">Номер сертификата</p>
            <p>{certificate.certificate_number}</p>
          </div>
        </div>

        <p className="certificate-verify-hint">
          Проверить подлинность:{" "}
          <Link href={`/certificates/verify/${certificate.certificate_number}`}>
            /certificates/verify/{certificate.certificate_number}
          </Link>
        </p>
      </div>
    </>
  );
}
