

import { BROWSER_APP_SDK_DEP_RANGE } from "@shared/browser-app-sdk-version";

export type ScaffoldTemplate = "react" | "vanilla";

export interface ScaffoldOptions {
  name: string;

  icon?: string;
  description?: string;
}

export interface ScaffoldFile {
  path: string;
  content: string;
}

function normalize(opts: ScaffoldOptions): {
  name: string;
  icon: string;
  description?: string;
} {
  return {
    name: (opts.name || "My App").trim() || "My App",
    icon: (opts.icon || "📦").trim() || "📦",
    description: opts.description?.trim() || undefined,
  };
}

function idaptManifest(n: {
  name: string;
  icon: string;
  description?: string;
}): Record<string, unknown> {
  const manifest: Record<string, unknown> = {
    entrypoint: "dist/index.html",
    name: n.name,
    icon: n.icon,
    version: "0.1.0",
    permissions: [],
  };
  if (n.description) manifest.description = n.description;
  return manifest;
}

//   - (react) an ESLint flat config with `eslint-plugin-jsx-a11y` so missing

const VITEST_CONFIG = `import { defineConfig } from "vitest/config";

// Unit tests run in Node — the mock Idapt SDK needs no DOM. Add
// \`environment: "jsdom"\` + @testing-library if you render components in tests.
export default defineConfig({
  test: {
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
  },
});
`;

const EXAMPLE_UNIT_TEST = `/**
 * Example unit test — runs WITHOUT a live Idapt origin via the mock harness.
 *
 * \`createMockIdapt()\` returns a fake Idapt client whose \`data\` is an in-memory
 * KV and whose \`app\` reads from a fixture map, so app logic built on
 * \`client.data\` / \`client.app\` is unit-testable offline (the real \`connect()\`
 * throws when run off an Idapt app subdomain).
 */
import { createMockIdapt } from "@idapt/browser-app-sdk/testing";
import { describe, expect, it } from "vitest";

describe("idapt mock harness", () => {
  it("round-trips per-user state through the in-memory data KV", async () => {
    const idapt = createMockIdapt();
    await idapt.data.setJSON("prefs.json", { theme: "dark" });
    expect(await idapt.data.getJSON("prefs.json")).toEqual({ theme: "dark" });
    expect(await idapt.data.get("missing.json")).toBeNull();
  });

  it("reads the app's own bundle from the fixture map", async () => {
    const idapt = createMockIdapt({
      files: { "idapt.json": JSON.stringify({ name: "Demo" }) },
    });
    const names = (await idapt.app.list()).map((f) => f.name);
    expect(names).toContain("idapt.json");
  });
});
`;

const PLAYWRIGHT_CONFIG = `import { defineConfig } from "@playwright/test";

// Point PREVIEW_URL at a deployment's preview (or a local \`vite preview\`) and run
// \`npm run test:e2e\`. The specs assert against the accessibility tree, so they
// read the app exactly as assistive tech and \`browser-app snapshot\` do.
export default defineConfig({
  testDir: "./e2e",
  use: { baseURL: process.env.PREVIEW_URL ?? "http://localhost:4173" },
});
`;

const E2E_SPEC = `import { expect, test } from "@playwright/test";

// Accessibility-first assertions: querying by ROLE (not a brittle CSS selector)
// is robust AND proves the app exposes a semantic tree — the same tree the agent
// sees via \`browser-app snapshot\`.
test("renders an accessible top-level heading", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
});
`;

const IDAPT_CI_YML = `version: 1
jobs:
  test:
    executor: cloud
    steps:
      - npm install
      - npm test
  build:
    executor: cloud
    needs: [test]
    steps:
      - npm install
      - npm run build
    output:
      paths:
        - dist/**
deploy:
  target: browser-app
  app: __APP_NAME__
  artifacts: build
`;

const ESLINT_A11Y_CONFIG = `import jsxA11y from "eslint-plugin-jsx-a11y";

// Accessibility-first: the SAME semantic tree powers assistive tech, the e2e
// specs, and the agent's \`browser-app snapshot\`. jsx-a11y flags missing labels,
// alt text, and invalid roles so inaccessible markup fails lint, not review.
export default [
  {
    files: ["src/**/*.{ts,tsx}"],
    plugins: { "jsx-a11y": jsxA11y },
    rules: jsxA11y.flatConfigs.recommended.rules,
  },
];
`;

const REACT_INDEX_HTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>__APP_NAME__</title>
  </head>
  <body>
    <div id="root"></div>
    <!-- npm-only: the app bundles @idapt/browser-app-sdk itself (a dependency).
         The SDK auto-connects via the app-key cookie; no script tag needed. -->
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`;

const REACT_PKG = "react";
const REACT_DOM_CLIENT_PKG = "react-dom/client";

const REACT_MAIN_TSX = `import { StrictMode } from "${REACT_PKG}";
import { createRoot } from "${REACT_DOM_CLIENT_PKG}";
import { App } from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
`;

const REACT_APP_TSX = `import { connect } from "@idapt/browser-app-sdk";
import { useEffect, useState } from "${REACT_PKG}";

export function App() {
  const [status, setStatus] = useState("Connecting…");

  useEffect(() => {
    // On an idapt app subdomain the app-key cookie auto-connects — no args.
    connect()
      .then(() => setStatus("Connected."))
      .catch(() => setStatus("Could not connect to idapt."));
  }, []);

  return (
    <main style={{ maxWidth: "36rem", margin: "0 auto", padding: "2rem 1.25rem" }}>
      <h1>__APP_NAME__</h1>
      <p>{status}</p>
    </main>
  );
}
`;

const REACT_VITE_CONFIG = `import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Build to dist/ (the served bundle). Relative base so assets resolve on the
// app subdomain regardless of mount path.
export default defineConfig({
  base: "./",
  plugins: [react()],
  build: { outDir: "dist" },
});
`;

const REACT_TSCONFIG = `{
  "compilerOptions": {
    "target": "ES2020",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "react-jsx",
    "strict": true,
    "skipLibCheck": true,
    "noEmit": true
  },
  "include": ["src"]
}
`;

function getReactScaffold(opts: ScaffoldOptions): ScaffoldFile[] {
  const n = normalize(opts);
  const pkg = {
    name: "idapt-react-app",
    private: true,
    version: "0.1.0",
    type: "module",
    scripts: {
      build: "vite build",
      dev: "vite",
      test: "vitest run",
      "test:e2e": "playwright test",
      lint: "eslint .",
    },
    dependencies: {
      "@idapt/browser-app-sdk": BROWSER_APP_SDK_DEP_RANGE,
      react: "^18.3.1",
      "react-dom": "^18.3.1",
    },
    devDependencies: {
      "@playwright/test": "^1.48.0",
      "@vitejs/plugin-react": "^4.3.1",
      eslint: "^9.13.0",
      "eslint-plugin-jsx-a11y": "^6.10.0",
      typescript: "^5.5.4",
      vite: "^5.4.0",
      vitest: "^2.1.0",
    },
    idapt: idaptManifest(n),
  };

  return [
    { path: "package.json", content: `${JSON.stringify(pkg, null, 2)}\n` },
    {
      path: "index.html",
      content: REACT_INDEX_HTML.replaceAll("__APP_NAME__", n.name),
    },
    { path: "vite.config.ts", content: REACT_VITE_CONFIG },
    { path: "vitest.config.ts", content: VITEST_CONFIG },
    { path: "playwright.config.ts", content: PLAYWRIGHT_CONFIG },
    { path: "eslint.config.js", content: ESLINT_A11Y_CONFIG },
    {
      path: "idapt-ci.yml",
      content: IDAPT_CI_YML.replaceAll("__APP_NAME__", n.name),
    },
    { path: "tsconfig.json", content: REACT_TSCONFIG },
    { path: "src/main.tsx", content: REACT_MAIN_TSX },
    {
      path: "src/App.tsx",
      content: REACT_APP_TSX.replaceAll("__APP_NAME__", n.name),
    },
    { path: "src/example.test.ts", content: EXAMPLE_UNIT_TEST },
    { path: "e2e/app.spec.ts", content: E2E_SPEC },
  ];
}

const VANILLA_INDEX_HTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>__APP_NAME__</title>
    <link rel="stylesheet" href="main.css" />
  </head>
  <body>
    <main class="app">
      <h1>__APP_ICON__ __APP_NAME__</h1>
      <p id="status">Connecting…</p>
    </main>
    <!-- npm-only: the app bundles @idapt/browser-app-sdk itself (a dependency). -->
    <script type="module" src="main.js"></script>
  </body>
</html>
`;

const VANILLA_MAIN_TS = `// __APP_NAME__ — a buildable idapt browser-app (esbuild).
import { connect } from "@idapt/browser-app-sdk";

const statusEl = document.getElementById("status");
function setStatus(text: string) {
  if (statusEl) statusEl.textContent = text;
}

// On an idapt app subdomain the app-key cookie auto-connects — no args.
connect()
  .then(() => setStatus("Connected."))
  .catch(() => setStatus("Could not connect to idapt."));

export {};
`;

const VANILLA_MAIN_CSS = `body {
  margin: 0;
  font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  background: #0b0b0f;
  color: #f4f4f5;
}
.app {
  max-width: 36rem;
  margin: 0 auto;
  padding: 2rem 1.25rem;
}
`;

const VANILLA_BUILD_SH = `set -e
mkdir -p dist
node_modules/.bin/esbuild src/main.ts --bundle --format=esm --outfile=dist/main.js
cp index.html dist/index.html
cp src/main.css dist/main.css
`;

function getVanillaScaffold(opts: ScaffoldOptions): ScaffoldFile[] {
  const n = normalize(opts);
  const pkg = {
    name: "idapt-vanilla-app",
    private: true,
    version: "0.1.0",
    type: "module",
    scripts: {

      build: "sh build.sh",
      test: "vitest run",
      "test:e2e": "playwright test",
    },
    dependencies: {
      "@idapt/browser-app-sdk": BROWSER_APP_SDK_DEP_RANGE,
    },
    devDependencies: {
      "@playwright/test": "^1.48.0",
      esbuild: "^0.23.0",
      typescript: "^5.5.4",
      vitest: "^2.1.0",
    },
    idapt: idaptManifest(n),
  };

  return [
    { path: "package.json", content: `${JSON.stringify(pkg, null, 2)}\n` },
    { path: "build.sh", content: VANILLA_BUILD_SH },
    { path: "vitest.config.ts", content: VITEST_CONFIG },
    { path: "playwright.config.ts", content: PLAYWRIGHT_CONFIG },
    {
      path: "idapt-ci.yml",
      content: IDAPT_CI_YML.replaceAll("__APP_NAME__", n.name),
    },
    {
      path: "index.html",
      content: VANILLA_INDEX_HTML.replaceAll("__APP_NAME__", n.name).replaceAll(
        "__APP_ICON__",
        n.icon,
      ),
    },
    {
      path: "src/main.ts",
      content: VANILLA_MAIN_TS.replaceAll("__APP_NAME__", n.name),
    },
    { path: "src/main.css", content: VANILLA_MAIN_CSS },
    { path: "src/example.test.ts", content: EXAMPLE_UNIT_TEST },
    { path: "e2e/app.spec.ts", content: E2E_SPEC },
  ];
}

export function getScaffoldFiles(
  template: ScaffoldTemplate,
  opts: ScaffoldOptions,
): ScaffoldFile[] {
  return template === "react"
    ? getReactScaffold(opts)
    : getVanillaScaffold(opts);
}
