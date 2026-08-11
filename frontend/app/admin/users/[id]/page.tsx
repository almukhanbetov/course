import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminGetUser } from "@/lib/admin-api";
import { UserEditForm } from "@/components/admin/UserEditForm";

export default async function AdminUserDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const user = await adminGetUser(token, id);
  if (!user) {
    notFound();
  }

  return (
    <div>
      <h1>Edit user</h1>
      <UserEditForm user={user} />
    </div>
  );
}
