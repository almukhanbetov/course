import type { Metadata } from "next";
import { CourseListing } from "@/components/CourseListing";

export const metadata: Metadata = {
  title: "Курсы — LMS Platform",
  description: "Каталог курсов по программированию, базам данных, DevOps и frontend-разработке с поиском, фильтрами и отзывами студентов.",
  alternates: {
    // Every filter combination (?q=...&level=...&sort=...) renders the same
    // page shell with different data — none of them should be indexed as a
    // separate URL, so every variant declares this bare path as canonical.
    canonical: "/courses",
  },
};

export default async function CoursesPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;

  return (
    <main>
      <h1>Курсы</h1>
      <CourseListing searchParams={params} basePath="/courses" />
    </main>
  );
}
