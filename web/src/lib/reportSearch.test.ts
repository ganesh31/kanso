import { describe, expect, test } from "vite-plus/test";
import { reportSearchSchema } from "./reportSearch";

describe("reportSearchSchema", () => {
  test("keeps a valid tf and as_of", () => {
    expect(reportSearchSchema.parse({ tf: "weekly", as_of: "2024-09-10" })).toEqual({
      tf: "weekly",
      as_of: "2024-09-10",
    });
  });

  test("drops an unknown tf or a malformed as_of instead of failing", () => {
    expect(reportSearchSchema.parse({ tf: "monthly", as_of: "10/09/2024" })).toEqual({});
  });

  test("both are optional", () => {
    expect(reportSearchSchema.parse({})).toEqual({});
  });
});
