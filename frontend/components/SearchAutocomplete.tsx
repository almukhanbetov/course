"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { getCourseSuggestions, type CourseSuggestion } from "@/lib/api";

const DEBOUNCE_MS = 250;

// SearchAutocomplete replaces CourseFilters' plain
// <input type="search" name="q"> with a client-enhanced version. It keeps
// the same name="q" inside the same GET <form>, so Enter with nothing
// highlighted (or JS disabled entirely) still falls back to the original
// full-page-navigation search this form was built around — autocomplete is
// additive, not a replacement for that flow (Stage 22B1 scope: enhance the
// existing input, don't redesign the courses page).
export function SearchAutocomplete({ defaultValue }: { defaultValue: string }) {
  const router = useRouter();
  const [value, setValue] = useState(defaultValue);
  const [suggestions, setSuggestions] = useState<CourseSuggestion[]>([]);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [highlighted, setHighlighted] = useState(-1);

  const wrapperRef = useRef<HTMLDivElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    function handleOutsideClick(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleOutsideClick);
    return () => document.removeEventListener("mousedown", handleOutsideClick);
  }, []);

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
      abortRef.current?.abort();
    };
  }, []);

  function scheduleSuggestions(query: string) {
    if (debounceRef.current) clearTimeout(debounceRef.current);

    const trimmed = query.trim();
    if (!trimmed) {
      // Requirement: never request suggestions for an empty query.
      abortRef.current?.abort();
      setLoading(false);
      setSuggestions([]);
      setOpen(false);
      setHighlighted(-1);
      return;
    }

    debounceRef.current = setTimeout(async () => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;

      setLoading(true);
      setOpen(true);
      try {
        const results = await getCourseSuggestions(trimmed, controller.signal);
        setSuggestions(results);
        setHighlighted(-1);
      } catch {
        // Fail-safe and quiet: a network/API error just means no
        // suggestions to show — never a noisy error banner over a search
        // box. A stale request aborted by a newer keystroke also lands
        // here and is likewise ignored.
        setSuggestions([]);
      } finally {
        setLoading(false);
      }
    }, DEBOUNCE_MS);
  }

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const next = e.target.value;
    setValue(next);
    scheduleSuggestions(next);
  }

  function selectSuggestion(s: CourseSuggestion) {
    setOpen(false);
    setSuggestions([]);
    setHighlighted(-1);
    router.push(`/courses/${s.id}`);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (!open) return;

    // Escape must dismiss the dropdown whenever it's open — including
    // while it's showing "Загрузка..."/"Ничего не найдено" (suggestions is
    // still empty in both those states) — so it's handled before the
    // suggestions.length guard below, not inside the same switch as the
    // list-navigation keys, which only make sense once there's a list.
    if (e.key === "Escape") {
      setOpen(false);
      setHighlighted(-1);
      return;
    }

    if (suggestions.length === 0) return;

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setHighlighted((prev) => (prev + 1) % suggestions.length);
        break;
      case "ArrowUp":
        e.preventDefault();
        setHighlighted((prev) => (prev <= 0 ? suggestions.length - 1 : prev - 1));
        break;
      case "Enter":
        // Only intercept Enter when a suggestion is actually highlighted —
        // otherwise let it fall through to the form's native submit, the
        // same full-search behavior this input has always had.
        if (highlighted >= 0) {
          e.preventDefault();
          selectSuggestion(suggestions[highlighted]);
        }
        break;
    }
  }

  return (
    <div className="search-autocomplete" ref={wrapperRef}>
      <input
        type="search"
        name="q"
        placeholder="Поиск по курсам..."
        aria-label="Поиск"
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={() => {
          if (suggestions.length > 0) setOpen(true);
        }}
        autoComplete="off"
        role="combobox"
        aria-expanded={open}
        aria-controls="search-autocomplete-list"
      />
      {open && (
        <ul id="search-autocomplete-list" className="search-autocomplete-dropdown" role="listbox">
          {loading ? (
            <li className="search-autocomplete-status" aria-live="polite">
              Загрузка...
            </li>
          ) : suggestions.length === 0 ? (
            <li className="search-autocomplete-status">Ничего не найдено</li>
          ) : (
            suggestions.map((s, i) => (
              <li key={s.id} role="option" aria-selected={i === highlighted}>
                <button
                  type="button"
                  className={`search-autocomplete-item${i === highlighted ? " is-highlighted" : ""}`}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => selectSuggestion(s)}
                  onMouseEnter={() => setHighlighted(i)}
                >
                  <span className="search-autocomplete-title">{s.title}</span>
                  {s.category_name && <span className="search-autocomplete-category">{s.category_name}</span>}
                </button>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}
