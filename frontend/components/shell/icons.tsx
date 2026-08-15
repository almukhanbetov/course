// Hand-rolled inline icon set (stroke-based, 18x18) — deliberately not an
// npm dependency, matching the project's "no icon library" stack. Every
// icon shares the same props/shape so callers can swap them interchangeably
// inside nav items.

export type IconProps = { size?: number; className?: string };

function base(children: React.ReactNode, { size = 18, className }: IconProps = {}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export const IconDashboard = (p: IconProps = {}) =>
  base(
    <>
      <rect x="3" y="3" width="7" height="9" rx="1.5" />
      <rect x="14" y="3" width="7" height="5" rx="1.5" />
      <rect x="14" y="12" width="7" height="9" rx="1.5" />
      <rect x="3" y="16" width="7" height="5" rx="1.5" />
    </>,
    p
  );

export const IconCourses = (p: IconProps = {}) =>
  base(
    <>
      <path d="M4 19.5V5.5A2 2 0 0 1 6 3.5h13a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1H6a2 2 0 0 0-2 2" />
      <path d="M4 19.5A2 2 0 0 1 6 17.5h14" />
    </>,
    p
  );

export const IconHeart = (p: IconProps = {}) =>
  base(<path d="M20.5 8.5c0 4.5-8.5 10-8.5 10s-8.5-5.5-8.5-10a4.5 4.5 0 0 1 8.5-2 4.5 4.5 0 0 1 8.5 2Z" />, p);

export const IconPlayCircle = (p: IconProps = {}) =>
  base(
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M10.5 8.5v7l6-3.5-6-3.5Z" fill="currentColor" stroke="none" />
    </>,
    p
  );

export const IconSparkles = (p: IconProps = {}) =>
  base(
    <>
      <path d="M11 3v3M11 15v3M3 9h3M17 9h4M6 6l2 2M14 6l-2 2M6 12l2-2M14 12l-2-2" />
      <circle cx="18" cy="17" r="2.2" />
    </>,
    p
  );

export const IconUser = (p: IconProps = {}) =>
  base(
    <>
      <circle cx="12" cy="8" r="3.6" />
      <path d="M4.5 20.2a7.5 7.5 0 0 1 15 0" />
    </>,
    p
  );

export const IconBell = (p: IconProps = {}) =>
  base(
    <>
      <path d="M6 9.5a6 6 0 0 1 12 0c0 4 1.5 5.5 1.5 5.5H4.5S6 13.5 6 9.5Z" />
      <path d="M10 18.5a2 2 0 0 0 4 0" />
    </>,
    p
  );

export const IconShield = (p: IconProps = {}) =>
  base(<path d="M12 3.5 5 6v6c0 4.5 3 7.5 7 8.5 4-1 7-4 7-8.5V6l-7-2.5Z" />, p);

export const IconGraduationCap = (p: IconProps = {}) =>
  base(
    <>
      <path d="M2.5 9 12 4.5 21.5 9 12 13.5 2.5 9Z" />
      <path d="M6.5 11v4.2c0 1.4 2.4 3 5.5 3s5.5-1.6 5.5-3V11" />
    </>,
    p
  );

export const IconMenu = (p: IconProps = {}) => base(<path d="M3.5 6.5h17M3.5 12h17M3.5 17.5h17" />, p);

export const IconClose = (p: IconProps = {}) => base(<path d="M5 5l14 14M19 5 5 19" />, p);

export const IconLogout = (p: IconProps = {}) =>
  base(
    <>
      <path d="M9 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h3" />
      <path d="M15.5 16.5 20 12l-4.5-4.5" />
      <path d="M20 12H9" />
    </>,
    p
  );

export const IconAward = (p: IconProps = {}) =>
  base(
    <>
      <circle cx="12" cy="8.5" r="5.5" />
      <path d="M8.5 13 7 21l5-2.5L17 21l-1.5-8" />
    </>,
    p
  );

export const IconCreditCard = (p: IconProps = {}) =>
  base(
    <>
      <rect x="3" y="5.5" width="18" height="13" rx="2" />
      <path d="M3 10h18" />
    </>,
    p
  );

export const IconBarChart = (p: IconProps = {}) =>
  base(
    <>
      <path d="M4 20V10M11 20V4M18 20v-7" />
    </>,
    p
  );

export const IconUsers = (p: IconProps = {}) =>
  base(
    <>
      <circle cx="9" cy="8" r="3.2" />
      <path d="M3.5 19a5.5 5.5 0 0 1 11 0" />
      <path d="M15.5 5.5a3.2 3.2 0 0 1 0 6.2" />
      <path d="M17 13.3a5.5 5.5 0 0 1 3.5 5.7" />
    </>,
    p
  );

export const IconClipboard = (p: IconProps = {}) =>
  base(
    <>
      <rect x="5.5" y="4.5" width="13" height="16" rx="2" />
      <path d="M9 4.5V3.8a1.3 1.3 0 0 1 1.3-1.3h3.4A1.3 1.3 0 0 1 15 3.8v.7" />
      <path d="M8.5 11h7M8.5 15h7" />
    </>,
    p
  );

export const IconLayers = (p: IconProps = {}) =>
  base(
    <>
      <path d="M12 3.5 21 9l-9 5.5L3 9l9-5.5Z" />
      <path d="M3 14.5 12 20l9-5.5" />
    </>,
    p
  );

export const IconTag = (p: IconProps = {}) =>
  base(
    <>
      <path d="M11.5 3.5H5a1.5 1.5 0 0 0-1.5 1.5v6.5L13 21l8-8-9.5-9.5Z" />
      <circle cx="8" cy="8" r="1.2" fill="currentColor" stroke="none" />
    </>,
    p
  );

export const IconStar = (p: IconProps = {}) =>
  base(<path d="m12 3.5 2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17.4l-5.4 2.9 1-6-4.4-4.4 6.1-.9L12 3.5Z" />, p);

export const IconFileText = (p: IconProps = {}) =>
  base(
    <>
      <path d="M7 3.5h7l4 4v13H7z" />
      <path d="M14 3.5v4h4M9.5 12h5M9.5 15.5h5" />
    </>,
    p
  );

export const IconSearch = (p: IconProps = {}) =>
  base(
    <>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m20 20-4.3-4.3" />
    </>,
    p
  );

export const IconMessageCircle = (p: IconProps = {}) =>
  base(
    <>
      <path d="M21 12a8.5 8.5 0 0 1-8.5 8.5c-1.3 0-2.5-.3-3.6-.8L3.5 21l1.4-4.6A8.5 8.5 0 1 1 21 12Z" />
      <path d="M8.5 12h.01M12 12h.01M15.5 12h.01" />
    </>,
    p
  );

export const IconFlag = (p: IconProps = {}) =>
  base(
    <>
      <path d="M5 3v18" />
      <path d="M5 4h13l-3 4 3 4H5" />
    </>,
    p
  );

// Vitals/pulse line — Stage 29A5's /admin/system-health nav entry.
// IconHeart already exists but is already the wishlist icon elsewhere in
// this app (app/dashboard/layout.tsx); reusing it here for an unrelated
// meaning would be confusing, so this is one more glyph in the same
// hand-rolled set rather than a new design system.
export const IconActivity = (p: IconProps = {}) =>
  base(<path d="M3 12h4l2.5-7 4 14 2.5-7H21" />, p);

// Error/alert states across the public pages (ErrorState component) — a
// calm outline glyph, not a filled red triangle, matching this set's
// restrained stroke style.
export const IconAlertCircle = (p: IconProps = {}) =>
  base(
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7.5v5.5" />
      <path d="M12 16.2h.01" />
    </>,
    p
  );

// Category-driven course-card placeholders (CourseCard) — one glyph per
// known backend category slug, generic/deterministic rather than a stand-in
// photo. Falls back to IconCourses for any category this set doesn't cover.
export const IconDatabase = (p: IconProps = {}) =>
  base(
    <>
      <ellipse cx="12" cy="5.5" rx="7.5" ry="2.8" />
      <path d="M4.5 5.5v6.5c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8V5.5" />
      <path d="M4.5 12v6.5c0 1.5 3.4 2.8 7.5 2.8s7.5-1.3 7.5-2.8V12" />
    </>,
    p
  );

export const IconServerStack = (p: IconProps = {}) =>
  base(
    <>
      <rect x="3.5" y="4" width="17" height="6" rx="1.5" />
      <rect x="3.5" y="14" width="17" height="6" rx="1.5" />
      <path d="M7 7h.01M7 17h.01" />
    </>,
    p
  );

export const IconCode = (p: IconProps = {}) =>
  base(<path d="M9 8 4.5 12 9 16M15 8l4.5 4-4.5 4" />, p);

export const IconBrowser = (p: IconProps = {}) =>
  base(
    <>
      <rect x="3" y="4.5" width="18" height="15" rx="1.8" />
      <path d="M3 8.5h18" />
      <path d="M6.3 6.5h.01" />
      <path d="M9 6.5h.01" />
    </>,
    p
  );
