import type { Metadata } from "next";
import { SpecialityForm } from "@/components/admin/SpecialityForm";

export const metadata: Metadata = {
  title: "New speciality — Admin",
};

export default function NewSpecialityPage() {
  return (
    <div>
      <h1>New speciality</h1>
      <SpecialityForm />
    </div>
  );
}
