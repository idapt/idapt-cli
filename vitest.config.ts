import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

const dir = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  test: {
    include: ["src/**/*.test.ts"],
    environment: "node",
  },
  resolve: {
    alias: {
      "@idapt/api-contracts": path.resolve(dir, "../api-contracts/src"),

      "@idapt/sdk": path.resolve(dir, "../sdk/src/index.ts"),
      "@shared": path.resolve(dir, "../../shared"),
    },
  },
});
