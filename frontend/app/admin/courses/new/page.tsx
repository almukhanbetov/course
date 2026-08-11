import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListCategories, adminListInstructors } from "@/lib/admin-api";
import { CourseForm } from "@/components/admin/CourseForm";

export const metadata: Metadata = {
  title: "New course — Admin",
};

export default async function NewCoursePage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const [categories, instructors] = await Promise.all([adminListCategories(token), adminListInstructors(token)]);

  return (
    <div>
      <h1>New course</h1>
      <CourseForm categories={categories} instructors={instructors} />
    </div>
  );
}
