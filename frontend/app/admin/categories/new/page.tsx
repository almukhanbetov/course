import type { Metadata } from "next";
import { CategoryForm } from "@/components/admin/CategoryForm";

export const metadata: Metadata = {
  title: "New category — Admin",
};

export default function NewCategoryPage() {
  return (
    <div>
      <h1>New category</h1>
      <CategoryForm />
    </div>
  );
}
