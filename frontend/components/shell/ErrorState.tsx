import { IconAlertCircle } from "@/components/shell/icons";

// Calm, consistent presentation for the "backend request failed" branches
// already present on every public page (CourseListing, /specialities,
// course/speciality detail) — purely visual, the error/loading logic that
// decides when to render this is unchanged.
export function ErrorState({ message }: { message: string }) {
  return (
    <div className="error-state" role="alert">
      <IconAlertCircle size={22} />
      <p>{message}</p>
    </div>
  );
}
