import type { Category } from "@/lib/api";
import { SearchAutocomplete } from "@/components/SearchAutocomplete";

interface Props {
  categories: Category[];
  basePath: string;
  current: { q: string; category: string; level: string; access_type: string; sort: string };
  showCategoryFilter: boolean;
}

// A plain GET <form> — filters only take effect on submit (Enter or the
// button), and the resulting URL is fully shareable/bookmarkable and works
// with the back button. The search input itself is progressively enhanced
// by SearchAutocomplete (Stage 22B1) with a suggestions dropdown, but stays
// inside this same form under the same name="q" — submitting still works
// exactly as before, autocomplete is purely additive.
export function CourseFilters({ categories, basePath, current, showCategoryFilter }: Props) {
  return (
    <form action={basePath} method="GET" className="filter-bar">
      <SearchAutocomplete defaultValue={current.q} />

      {showCategoryFilter && (
        <select name="category" defaultValue={current.category} aria-label="Категория">
          <option value="">Все категории</option>
          {categories.map((c) => (
            <option key={c.id} value={c.slug}>
              {c.name}
            </option>
          ))}
        </select>
      )}

      <select name="level" defaultValue={current.level} aria-label="Уровень">
        <option value="">Любой уровень</option>
        <option value="beginner">Начальный</option>
        <option value="intermediate">Средний</option>
        <option value="advanced">Продвинутый</option>
      </select>

      <select name="access_type" defaultValue={current.access_type} aria-label="Доступ">
        <option value="">Любой доступ</option>
        <option value="free">Бесплатный</option>
        <option value="subscription">По подписке</option>
      </select>

      <select name="sort" defaultValue={current.sort} aria-label="Сортировка">
        <option value="">По умолчанию</option>
        <option value="relevance">По релевантности</option>
        <option value="newest">Сначала новые</option>
        <option value="rating">По рейтингу</option>
        <option value="title">По названию</option>
      </select>

      <button type="submit" className="btn-primary">
        Найти
      </button>
    </form>
  );
}
