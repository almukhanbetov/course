import "server-only";
import { cookies } from "next/headers";
import { SERVER_API_URL, type PublicUser } from "@/lib/api";

export const SESSION_COOKIE = "lms_session";
export const SESSION_MAX_AGE = 60 * 60 * 24; // 24h, matches backend access token TTL

export async function getSessionToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(SESSION_COOKIE)?.value ?? null;
}

export async function getCurrentUser(): Promise<PublicUser | null> {
  const token = await getSessionToken();
  if (!token) {
    return null;
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/me`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    return null;
  }

  return res.json();
}
