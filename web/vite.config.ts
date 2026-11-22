import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite-plus";

const api = "http://localhost:8080";

export default defineConfig({
  plugins: [tanstackRouter({ target: "react", autoCodeSplitting: true }), react(), tailwindcss()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  server: {
    proxy: { "/api": api, "/docs": api, "/openapi.json": api },
  },
  lint: {
    ignorePatterns: [
      "dist/**",
      "playwright-report/**",
      "test-results/**",
      "src/api/generated.ts",
      "src/routeTree.gen.ts",
    ],
    options: { typeAware: true, typeCheck: true },
  },
  fmt: {
    printWidth: 120,
    ignorePatterns: [
      "playwright-report/**",
      "test-results/**",
      "src/api/generated.ts",
      "src/routeTree.gen.ts",
      "pnpm-lock.yaml",
    ],
  },
  test: {
    // Unit tests only; e2e/ is Playwright's (pnpm exec playwright test).
    include: ["src/**/*.test.{ts,tsx}"],
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
});
