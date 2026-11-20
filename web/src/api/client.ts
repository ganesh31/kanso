import createClient from "openapi-fetch";
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type Report = Schemas["Report"];
export type ReportLine = Schemas["ReportLine"];
export type Evidence = Schemas["Evidence"];
export type EvidenceType = Evidence["type"];
export type PatternMeta = Schemas["Meta"];
export type Bar = Schemas["Bar"];
export type ReportOptions = Schemas["ReportRequest"];

export interface FieldError {
  /** Path in the request body, e.g. "entry.any". */
  path: string;
  message: string;
}

/** Every failed API call throws this. `fields` maps huma's error locations to body paths. */
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

type Problem = Partial<Schemas["ErrorModel"]>;

function toApiError(status: number, body: unknown): ApiError {
  const problem = (typeof body === "object" && body !== null ? body : {}) as Problem;
  const fields = (problem.errors ?? []).map((e) => ({
    path: (e.location ?? "").replace(/^body\.?/, ""),
    message: e.message ?? "",
  }));
  return new ApiError(status, problem.detail || problem.title || `HTTP ${status}`, fields);
}

async function unwrap<T>(
  call: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await call;
  if (!response.ok || data === undefined) {
    throw toApiError(response.status, error);
  }
  return data;
}

export interface ApiOptions {
  baseUrl?: string;
  fetch?: (req: Request) => Promise<Response>;
}

export function createApi(opts: ApiOptions = {}) {
  const client = createClient<paths>({
    baseUrl: opts.baseUrl ?? (typeof window === "undefined" ? "" : window.location.origin),
    ...(opts.fetch ? { fetch: opts.fetch } : {}),
  });

  return {
    patterns: async () => (await unwrap(client.GET("/api/patterns"))).patterns,
    report: (symbol: string, options: ReportOptions = {}) =>
      unwrap(
        client.POST("/api/report/{symbol}", {
          params: { path: { symbol } },
          body: options,
        }),
      ),
    bars: (symbol: string, range: { from?: string; to?: string } = {}) =>
      unwrap(
        client.GET("/api/bars/{symbol}", {
          params: { path: { symbol }, query: range },
        }),
      ),
  };
}

export type Api = ReturnType<typeof createApi>;

export const api: Api = createApi();
