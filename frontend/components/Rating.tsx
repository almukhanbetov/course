import { IconStar } from "@/components/shell/icons";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { DEFAULT_LOCALE, type Locale } from "@/lib/i18n/locale";

// locale defaults to Russian so call sites outside the translated public
// pages (e.g. the student dashboard) don't need to thread it through just
// to keep compiling.
export function Rating({ average, count, locale = DEFAULT_LOCALE }: { average: number; count: number; locale?: Locale }) {
  if (count === 0) {
    return (
      <span className="rating rating-empty">
        <IconStar size={13} />
        {getDictionary(locale).courses.noReviews}
      </span>
    );
  }

  return (
    <span className="rating">
      ★ {average.toFixed(1)} <span className="rating-count">({count})</span>
    </span>
  );
}
