import type { Category } from "@/lib/api";
import { SearchAutocomplete } from "@/components/SearchAutocomplete";
import { IconBarChart, IconSearch, IconShield, IconSort, IconTag } from "@/components/shell/icons";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { localizeName } from "@/lib/i18n/localize";
import type { Locale } from "@/lib/i18n/locale";

interface Props {
  categories: Category[];
  basePath: string;
  current: { q: string; category: string; level: string; access_type: string; sort: string };
  showCategoryFilter: boolean;
  locale: Locale;
  // "bar": original horizontal single-row form (default, used by
  // /categories/[slug]). "sidebar": same fields/form, stacked vertically
  // with labels for the /courses left filter panel.
  layout?: "bar" | "sidebar";
}

// A plain GET <form> — filters only take effect on submit (Enter or the
// button), and the resulting URL is fully shareable/bookmarkable and works
// with the back button. The search input itself is progressively enhanced
// by SearchAutocomplete (Stage 22B1) with a suggestions dropdown, but stays
// inside this same form under the same name="q" — submitting still works
// exactly as before, autocomplete is purely additive.
export function CourseFilters({ categories, basePath, current, showCategoryFilter, locale, layout = "bar" }: Props) {
  const sidebar = layout === "sidebar";
  const dict = getDictionary(locale).courses;
  const field = (label: string, control: React.ReactNode, icon?: React.ReactNode) =>
    sidebar ? (
      <label className="filter-field">
        <span>
          {icon}
          {label}
        </span>
        {control}
      </label>
    ) : (
      control
    );

  return (
    <form action={basePath} method="GET" className={sidebar ? "filter-bar filter-bar-sidebar" : "filter-bar"}>
      {field(dict.searchLabel, <SearchAutocomplete defaultValue={current.q} locale={locale} />, <IconSearch size={14} />)}

      {showCategoryFilter &&
        field(
          dict.categoryLabel,
          <select name="category" defaultValue={current.category} aria-label={dict.categoryLabel}>
            <option value="">{dict.allCategories}</option>
            {categories.map((c) => (
              <option key={c.id} value={c.slug}>
                {localizeName(c, locale)}
              </option>
            ))}
          </select>,
          <IconTag size={14} />
        )}

      {field(
        dict.levelLabel,
        <select name="level" defaultValue={current.level} aria-label={dict.levelLabel}>
          <option value="">{dict.anyLevel}</option>
          <option value="beginner">{dict.levelBeginner}</option>
          <option value="intermediate">{dict.levelIntermediate}</option>
          <option value="advanced">{dict.levelAdvanced}</option>
        </select>,
        <IconBarChart size={14} />
      )}

      {field(
        dict.accessLabel,
        <select name="access_type" defaultValue={current.access_type} aria-label={dict.accessLabel}>
          <option value="">{dict.anyAccess}</option>
          <option value="free">{dict.accessFree}</option>
          <option value="subscription">{dict.accessSubscription}</option>
        </select>,
        <IconShield size={14} />
      )}

      {field(
        dict.sortLabel,
        <select name="sort" defaultValue={current.sort} aria-label={dict.sortLabel}>
          <option value="">{dict.sortDefault}</option>
          <option value="relevance">{dict.sortRelevance}</option>
          <option value="newest">{dict.sortNewest}</option>
          <option value="rating">{dict.sortRating}</option>
          <option value="title">{dict.sortTitle}</option>
        </select>,
        <IconSort size={14} />
      )}

      <button type="submit" className="btn-primary">
        {dict.submitButton}
      </button>
    </form>
  );
}
