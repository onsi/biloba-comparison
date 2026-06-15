import { defineConfig } from '@playwright/test';

// Read the shared target from the environment; never hard-code a port.
const baseURL = process.env.BASE_URL ?? 'http://127.0.0.1:9889';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  retries: 0,
  // Workers come from the CLI (`--workers=N`); do not hard-pin them.
  reporter: 'line',
  use: {
    baseURL,
    headless: true, // drives chromium-headless-shell
    // No artifacts — video/trace/screenshots are asymmetric overhead.
    video: 'off',
    trace: 'off',
    screenshot: 'off',
  },
});
