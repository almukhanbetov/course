import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { getMyWishlist } from "@/lib/api";
import { WishlistButton } from "@/components/WishlistButton";
import { Rating } from "@/components/Rating";

export const metadata: Metadata = {
  title: "Избранное — LMS Platform",
};

export default async function WishlistPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const items = await getMyWishlist(token).catch(() => []);

  return (
    <main>
      <Link href="/dashboard">← Личный кабинет</Link>
      <h1>Избранное</h1>

      {items.length === 0 ? (
        <div className="empty-state">
          <p>В избранном пока нет курсов.</p>
          <Link href="/courses" className="btn-secondary">
            Смотреть каталог курсов
          </Link>
        </div>
      ) : (
        <div className="course-grid">
          {items.map((item) => (
            <div key={item.course_id} className="course-card">
              <WishlistButton courseId={item.course_id} initialInWishlist={true} />
              <Link href={`/courses/${item.course_id}`} className="course-card-link">
                <div className="course-card-image">
                  {item.image_url ? <img src={item.image_url} alt="" /> : "Нет изображения"}
                </div>
                <h2>{item.title}</h2>
                {item.category_name && <span className="badge badge-category">{item.category_name}</span>}{" "}
                <span className={`badge ${item.access_type === "free" ? "badge-free" : "badge-premium"}`}>
                  {item.access_type === "free" ? "Бесплатный" : "По подписке"}
                </span>
                <div className="course-card-rating">
                  <Rating average={item.rating_average} count={item.rating_count} />
                </div>
              </Link>
            </div>
          ))}
        </div>
      )}
    </main>
  );
}
