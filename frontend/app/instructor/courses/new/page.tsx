import type { Metadata } from "next";
import { getCategories } from "@/lib/api";
import { InstructorCourseForm } from "@/components/instructor/InstructorCourseForm";

export const metadata: Metadata = {
  title: "New Course — Instructor",
};

export default async function NewInstructorCoursePage() {
  const categories = await getCategories();

  return (
    <div>
      <h1>Новый курс</h1>
      <InstructorCourseForm categories={categories} />
    </div>
  );
}
