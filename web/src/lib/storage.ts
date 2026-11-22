/**
 * localStorage with every access wrapped: a private window, blocked site data or a full quota
 * never throws. A value that could not be written is kept in memory for the rest of the session,
 * and reads prefer it, so the app behaves the same until the page is reloaded.
 */

const memory = new Map<string, string>();

export function readItem(key: string): string | null {
  const unsaved = memory.get(key);
  if (unsaved !== undefined) return unsaved;
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function writeItem(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
    memory.delete(key);
  } catch {
    memory.set(key, value);
  }
}

/** Forgets the in-memory values (tests start each case clean). */
export function resetMemoryStorage(): void {
  memory.clear();
}
