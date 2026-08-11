import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminGetSpeciality, adminListCourses } from "@/lib/admin-api";
import { SpecialityForm } from "@/components/admin/SpecialityForm";
import { AddCourseToRoadmapForm } from "@/components/admin/AddCourseToRoadmapForm";
import { ConfirmButton } from "@/components/ConfirmButton";
import {
  deleteSpecialityAction,
  removeCourseFromSpecialityAction,
  moveSpecialityCourseAction,
  toggleRequiredAction,
} from "@/lib/admin-actions";

export default async function AdminSpecialityDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const speciality = await adminGetSpeciality(token, id);
  if (!speciality) {
    notFound();
  }

  const allCourses = await adminListCourses(token, { limit: 100 });
  const roadmapCourseIds = new Set(speciality.courses.map((c) => c.id));
  const availableCourses = allCourses.items.filter((c) => !roadmapCourseIds.has(c.id));
  const courses = [...speciality.courses].sort((a, b) => a.position - b.position);

  return (
    <div>
      <Link href="/admin/specialities">← All specialities</Link>
      <h1>{speciality.title}</h1>

      <div className="admin-card">
        <h2>Details</h2>
        <SpecialityForm speciality={speciality} />
        <form action={deleteSpecialityAction.bind(null, speciality.id)} className="mt-3">
          <ConfirmButton className="btn-danger" confirmMessage="Delete this speciality?">
            Delete speciality
          </ConfirmButton>
        </form>
      </div>

      <h2>Roadmap</h2>
      {courses.length === 0 && <p>No courses in this roadmap yet.</p>}

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Course</th>
              <th>Required</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {courses.map((course, idx) => (
              <tr key={course.id}>
                <td>{course.position}</td>
                <td>{course.title}</td>
                <td>
                  <form action={toggleRequiredAction.bind(null, speciality.id, course.id)}>
                    <button type="submit" className="btn-small">
                      {course.required ? "required" : "optional"} (toggle)
                    </button>
                  </form>
                </td>
                <td>
                  <div className="admin-inline-actions">
                    <form action={moveSpecialityCourseAction.bind(null, speciality.id, course.id, "up")}>
                      <button type="submit" className="btn-small" disabled={idx === 0}>
                        ↑
                      </button>
                    </form>
                    <form action={moveSpecialityCourseAction.bind(null, speciality.id, course.id, "down")}>
                      <button type="submit" className="btn-small" disabled={idx === courses.length - 1}>
                        ↓
                      </button>
                    </form>
                    <form action={removeCourseFromSpecialityAction.bind(null, speciality.id, course.id)}>
                      <ConfirmButton className="btn-danger" confirmMessage="Remove this course from the roadmap?">
                        Remove
                      </ConfirmButton>
                    </form>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {availableCourses.length > 0 && (
        <div className="admin-card">
          <h3>Add course to roadmap</h3>
          <AddCourseToRoadmapForm specialityId={speciality.id} courses={availableCourses} />
        </div>
      )}
    </div>
  );
}
