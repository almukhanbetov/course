import { getSessionToken } from "@/lib/session";
import { SERVER_API_URL } from "@/lib/api";

// This route exists for exactly one reason: the browser's native <video>
// tag and hls.js issue plain same-origin GET requests for playlists and
// segments — they cannot attach a custom Authorization header, and the
// session token lives in an httpOnly cookie client-side JS can't read
// anyway. So this route (same-origin to the browser, cookie-authenticated
// automatically) reads that cookie server-side, attaches it as a Bearer
// token, and forwards to the Go backend's own authorized proxy
// (/api/v1/video-stream/...), which re-checks enrollment/access on every
// single request. This route does no authorization itself and trusts
// nothing from the client beyond the path segments Next.js already parsed —
// the backend is the only source of truth for path validation and access.
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ videoId: string; path: string[] }> },
) {
  const { videoId, path } = await params;

  const token = await getSessionToken();
  if (!token) {
    return new Response("Unauthorized", { status: 401 });
  }

  const upstreamURL = `${SERVER_API_URL}/api/v1/video-stream/${encodeURIComponent(videoId)}/${path
    .map(encodeURIComponent)
    .join("/")}`;

  const upstream = await fetch(upstreamURL, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  const headers = new Headers();
  const contentType = upstream.headers.get("content-type");
  if (contentType) headers.set("content-type", contentType);
  const contentLength = upstream.headers.get("content-length");
  if (contentLength) headers.set("content-length", contentLength);

  return new Response(upstream.body, { status: upstream.status, headers });
}
