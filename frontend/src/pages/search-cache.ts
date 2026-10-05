import type { Profile } from '../api/types';

export type SearchField = 'name' | 'phone' | 'username';

export interface SearchState {
  field: SearchField;
  term: string;
  results: Profile[] | null;
}

const empty: SearchState = { field: 'name', term: '', results: null };

// Kept in memory rather than the URL so search terms (PII) stay out of browser
// history and server logs, while surviving navigation back from a profile.
let current: SearchState = empty;

/** Returns the last search so the page can restore it. */
export function getLastSearch(): SearchState {
  return current;
}

/** Stores the latest search. */
export function setLastSearch(s: SearchState): void {
  current = s;
}

/** Clears cached search results, e.g. when the session ends. */
export function clearSearchCache(): void {
  current = empty;
}
