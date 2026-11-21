/**
 * The one HTTP path for every generated API call (orval "mutator").
 * Every non-2xx response throws ApiError, with huma's error locations mapped to body field paths.
 */

export interface FieldError {
  /** Path in the request body, e.g. "entry.any". */
  path: string;
  message: string;
}

export class ApiError extends Error {
  readonly status: number;
  readonly fields: FieldError[];

  constructor(status: number, message: string, fields: FieldError[] = []) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.fields = fields;
  }
}

interface Problem {
  title?: string;
  detail?: string;
  errors?: { message?: string; location?: string }[] | null;
}

function toApiError(status: number, body: unknown): ApiError {
  const problem = (typeof body === "object" && body !== null ? body : {}) as Problem;
  const fields = (problem.errors ?? []).map((e) => ({
    path: (e.location ?? "").replace(/^body\.?/, ""),
    message: e.message ?? "",
  }));
  return new ApiError(status, problem.detail || problem.title || `HTTP ${status}`, fields);
}

export interface ApiConfig {
  /** Origin the relative API paths are resolved against. Defaults to the page origin. */
  baseUrl?: string;
  fetch?: typeof fetch;
}

let config: ApiConfig = {};

/** Point the generated client at another origin or fetch (tests use this). */
export function configureApi(next: ApiConfig): void {
  config = next;
}

export async function apiFetch<T>(url: string, init?: RequestInit): Promise<T> {
  const base = config.baseUrl ?? (typeof window === "undefined" ? "" : window.location.origin);
  const doFetch = config.fetch ?? fetch;
  const res = await doFetch(new Request(base + url, init));
  const text = await res.text();
  if (!res.ok) throw toApiError(res.status, parseOrUndefined(text));
  return (text ? JSON.parse(text) : undefined) as T;
}

/** Error bodies may be plain text (a proxy's 502), so a parse failure just means "no problem JSON". */
function parseOrUndefined(text: string): unknown {
  try {
    return text ? JSON.parse(text) : undefined;
  } catch {
    return undefined;
  }
}

/** orval types every generated hook's error with this, so `error` is always an ApiError. */
export type ErrorType<_E> = ApiError;
