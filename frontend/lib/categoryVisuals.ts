import { IconBrowser, IconCode, IconCourses, IconDatabase, IconServerStack } from "@/components/shell/icons";

export interface CategoryVisual {
  Icon: typeof IconCourses;
  accent: string;
}

// Deterministic icon + accent per known backend category slug, used for the
// course-card image placeholder when a course has no real image_url — never
// a stand-in photo, just a category-appropriate glyph. Falls back to the
// generic course icon for any category slug outside this set.
const CATEGORY_VISUALS: Record<string, CategoryVisual> = {
  programming: { Icon: IconCode, accent: "var(--accent)" },
  databases: { Icon: IconDatabase, accent: "var(--info)" },
  devops: { Icon: IconServerStack, accent: "var(--warning)" },
  frontend: { Icon: IconBrowser, accent: "#9d7bff" },
};

const FALLBACK_VISUAL: CategoryVisual = { Icon: IconCourses, accent: "var(--text-faint)" };

export function getCategoryVisual(categorySlug: string | undefined): CategoryVisual {
  return (categorySlug && CATEGORY_VISUALS[categorySlug]) || FALLBACK_VISUAL;
}
