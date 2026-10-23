import { defineConfig } from "../apps/web/node_modules/@playwright/test/index.js";
import { resolve } from "node:path";
export default defineConfig({
  testDir: "../apps/web/tests/e2e",
  testMatch: "deployment.spec.ts",
  outputDir: "../.tools/deploy-prep/focused-browser",
  reporter: "line",
  workers: 1,
  retries: 0,
  timeout: 20000,
  webServer: {
    cwd: resolve(__dirname, ".."),
    command:
      "npm --prefix apps/web run dev -- --host 127.0.0.1 --port 5184 --strictPort",
    url: "http://127.0.0.1:5184/pricing",
    reuseExistingServer: !process.env.CI,
  },
  use: { baseURL: "http://127.0.0.1:5184" },
  projects: [
    {
      name: "desktop",
      use: { browserName: "chromium", viewport: { width: 1440, height: 1000 } },
    },
    {
      name: "mobile",
      use: {
        browserName: "chromium",
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
      },
    },
  ],
});
