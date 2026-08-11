import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // Server Actions default to a 1MB body limit, far too small for a video
  // upload (uploadLessonVideoAction in lib/admin-actions.ts posts the file
  // through a bound Server Action). This is a dev-friendly ceiling, not the
  // authoritative one — VIDEO_MAX_UPLOAD_MB on the backend is what's
  // actually enforced.
  experimental: {
    serverActions: {
      bodySizeLimit: "512mb",
    },
  },
};

export default nextConfig;
