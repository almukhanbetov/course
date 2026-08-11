import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorGetCourse } from "@/lib/instructor-api";
import { InstructorTestForm } from "@/components/instructor/InstructorTestForm";

export default async function NewInstructorTestPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const course = await instructorGetCourse(token, id);
  if (!course) {
    notFound();
  }

  return (
    <div>
      <Link href={`/instructor/courses/${id}`}>← {course.title}</Link>
      <h1>Новый тест</h1>
      <InstructorTestForm courseId={id} />
    </div>
  );
}
