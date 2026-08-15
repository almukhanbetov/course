import { IconStar } from "@/components/shell/icons";

export function Rating({ average, count }: { average: number; count: number }) {
  if (count === 0) {
    return (
      <span className="rating rating-empty">
        <IconStar size={13} />
        Пока нет отзывов
      </span>
    );
  }

  return (
    <span className="rating">
      ★ {average.toFixed(1)} <span className="rating-count">({count})</span>
    </span>
  );
}
