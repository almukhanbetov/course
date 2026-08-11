import type { Metadata } from "next";
import { verifyCertificate } from "@/lib/api";

export const metadata: Metadata = {
  title: "Проверка сертификата — LMS Platform",
};

export default async function VerifyCertificatePage({
  params,
}: {
  params: Promise<{ certificateNumber: string }>;
}) {
  const { certificateNumber } = await params;

  let result: Awaited<ReturnType<typeof verifyCertificate>>;
  let error: string | null = null;

  try {
    result = await verifyCertificate(certificateNumber);
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
    result = { valid: false };
  }

  return (
    <main>
      <h1>Проверка сертификата</h1>
      <p className="my-course-meta">{certificateNumber}</p>

      {error && <p role="alert">Не удалось проверить сертификат: {error}</p>}

      {!error && result.valid && (
        <div className="status">
          <p className="success">✓ Сертификат действителен</p>
          <p>
            <strong>Студент:</strong> {result.student_name}
          </p>
          <p>
            <strong>Курс:</strong> {result.course_title}
          </p>
          <p>
            <strong>Дата выдачи:</strong>{" "}
            {result.issued_at && new Date(result.issued_at).toLocaleDateString("ru-RU")}
          </p>
        </div>
      )}

      {!error && !result.valid && (
        <div className="status">
          <p role="alert">Сертификат не найден</p>
        </div>
      )}
    </main>
  );
}
