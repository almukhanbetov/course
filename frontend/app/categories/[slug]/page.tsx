import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getCategories } from "@/lib/api";
import { CourseListing } from "@/components/CourseListing";

interface PageProps {
  params: Promise<{ slug: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { slug } = await params;
  const categories = await getCategories().catch(() => []);
  const category = categories.find((c) => c.slug === slug);

  if (!category) {
    return { title: "Категория не найдена — LMS Platform" };
  }

  return {
    title: `${category.name} — Курсы — LMS Platform`,
    description: category.description || `Курсы в категории «${category.name}».`,
    alternates: {
      // Filter params (?q=, ?level=, ?sort=, ...) never get their own
      // indexed URL — they all canonicalize back to the bare category page.
      canonical: `/categories/${category.slug}`,
    },
  };
}

export default async function CategoryPage({ params, searchParams }: PageProps) {
  const { slug } = await params;
  const search = await searchParams;

  const categories = await getCategories().catch(() => []);
  const category = categories.find((c) => c.slug === slug);

  if (!category) {
    notFound();
  }

  return (
    <main>
      <h1>{category.name}</h1>
      {category.description && <p className="subtitle">{category.description}</p>}
      <CourseListing searchParams={search} basePath={`/categories/${category.slug}`} fixedCategory={category.slug} />
    </main>
  );
}
