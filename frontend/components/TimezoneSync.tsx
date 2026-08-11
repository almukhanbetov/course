"use client";

import { useEffect } from "react";
import { updateTimezoneAction } from "@/lib/actions";

// Item 7: "Frontend может установить timezone браузера через обычный
// profile update." This runs once per mount, compares the browser's own
// resolved IANA zone against what the server already has on file, and only
// calls the update action when they differ — not on every render, and
// never more than once per page load.
export function TimezoneSync({ currentTimezone }: { currentTimezone: string }) {
  useEffect(() => {
    let browserTimezone: string;
    try {
      browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    } catch {
      return;
    }
    if (!browserTimezone || browserTimezone === currentTimezone) {
      return;
    }
    void updateTimezoneAction(browserTimezone);
  }, [currentTimezone]);

  return null;
}
