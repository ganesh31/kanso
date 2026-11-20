import { describe, expect, test } from "vitest";
import { ApiError, createApi } from "./client";

function respond(status: number, body: unknown) {
  return async () =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": status >= 400 ? "application/problem+json" : "application/json" },
    });
}

describe("api client", () => {
  test("client maps 400 body to ApiError with fields", async () => {
    const api = createApi({
      baseUrl: "http://test",
      fetch: respond(400, {
        title: "Bad Request",
        status: 400,
        detail: "validation failed",
        errors: [
          { message: "expected a date", location: "body.as_of", value: "2024-13-45" },
          { message: "need two conditions", location: "body.entry.any" },
        ],
      }),
    });
    const err = await api.report("TCS", { as_of: "2024-13-45" }).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    const apiErr = err as ApiError;
    expect(apiErr.status).toBe(400);
    expect(apiErr.message).toBe("validation failed");
    expect(apiErr.fields).toEqual([
      { path: "as_of", message: "expected a date" },
      { path: "entry.any", message: "need two conditions" },
    ]);
  });

  test("404 becomes ApiError without fields", async () => {
    const api = createApi({
      baseUrl: "http://test",
      fetch: respond(404, { title: "Not Found", status: 404, detail: "unknown symbol NOPE" }),
    });
    await expect(api.report("NOPE")).rejects.toMatchObject({
      status: 404,
      message: "unknown symbol NOPE",
      fields: [],
    });
  });

  test("report posts the options and returns the body", async () => {
    let seen: Request | undefined;
    const api = createApi({
      baseUrl: "http://test",
      fetch: async (req: Request) => {
        seen = req;
        return respond(200, { symbol: "TCS", as_of: "2024-01-05", timeframe: "daily", lines: [] })();
      },
    });
    const rep = await api.report("TCS", { as_of: "2024-01-05" });
    expect(rep.symbol).toBe("TCS");
    expect(seen?.method).toBe("POST");
    expect(seen?.url).toBe("http://test/api/report/TCS");
    expect(await seen?.json()).toEqual({ as_of: "2024-01-05" });
  });

  test("symbols are URL-encoded", async () => {
    let url = "";
    const api = createApi({
      baseUrl: "http://test",
      fetch: async (req: Request) => {
        url = req.url;
        return respond(200, { symbol: "^NSEI", bars: [] })();
      },
    });
    await api.bars("^NSEI");
    expect(url).toBe("http://test/api/bars/%5ENSEI");
  });
});
