import { defineConfig } from "../../../vendor/dreamrefactory/node_modules/vite";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { restoredSaveFormat } from "./save-format-plugin";

const root = fileURLToPath(new URL(".", import.meta.url));
const vendor = resolve(root, "../../../vendor/dreamrefactory");
const runtimeOnly = process.env.TITANIC_RUNTIME_ONLY === "1";
export default defineConfig({
  root,
  base: "./",
  publicDir: false,
  plugins: [restoredSaveFormat()],
  resolve: {
    alias: [
      ...(runtimeOnly ? [{ find: "@dreamfactory/taoot/cursor-art", replacement: resolve(root, "neutral-cursors.ts") }] : []),
      { find: "@dreamfactory/engine/df/savegame", replacement: resolve(root, "save-format.ts") },
      { find: /^@dreamfactory\/engine\/(.*)$/, replacement: `${vendor}/engine/src/$1` },
      { find: /^@dreamfactory\/site\/(.*)$/, replacement: `${vendor}/site/src/$1` },
      { find: /^@dreamfactory\/taoot\/(.*)$/, replacement: `${vendor}/taoot/src/$1` },
    ],
  },
  build: { outDir: resolve(root, runtimeOnly ? "dist-public" : "dist"), emptyOutDir: true, target: "safari17", sourcemap: true, modulePreload: false },
});
