"use client";

import { useState, useTransition } from "react";
import { addToWishlistAction, removeFromWishlistAction } from "@/lib/actions";

// Optimistic toggle: flips immediately, reconciles with whatever the
// backend actually persisted, reverts on error. Always calls
// preventDefault/stopPropagation since this button sits inside (or beside)
// a course card that's otherwise a single big <Link> — without it, a click
// would also navigate to the course page.
export function WishlistButton({ courseId, initialInWishlist }: { courseId: string; initialInWishlist: boolean }) {
  const [inWishlist, setInWishlist] = useState(initialInWishlist);
  const [pending, startTransition] = useTransition();

  function handleClick(e: React.MouseEvent<HTMLButtonElement>) {
    e.preventDefault();
    e.stopPropagation();

    const next = !inWishlist;
    setInWishlist(next);
    startTransition(async () => {
      const result = next ? await addToWishlistAction(courseId) : await removeFromWishlistAction(courseId);
      if (result.error || result.in_wishlist === undefined) {
        setInWishlist(!next);
        return;
      }
      setInWishlist(result.in_wishlist);
    });
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={pending}
      className={`wishlist-toggle-btn${inWishlist ? " active" : ""}`}
      aria-pressed={inWishlist}
      aria-label={inWishlist ? "Убрать из избранного" : "Добавить в избранное"}
    >
      {inWishlist ? "★ В избранном" : "☆ В избранное"}
    </button>
  );
}
