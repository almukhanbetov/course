import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminGetCategory } from "@/lib/admin-api";
import { CategoryForm } from "@/components/admin/CategoryForm";

export default async function AdminCategoryDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const category = await adminGetCategory(token, id);
  if (!category) {
    notFound();
  }

  return (
    <div>
      <Link href="/admin/categories">← All categories</Link>
      <h1>{category.name}</h1>

      <div className="admin-card">
        <h2>Details</h2>
        <CategoryForm category={category} />
        <p className="subtitle mt-3">
          There is no delete — a category referenced by courses must never disappear from under them. Uncheck
          &quot;Active&quot; to retire it from the public catalog instead.
        </p>
      </div>
    </div>
  );
}
