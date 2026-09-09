import { defineConfig } from "../../../vendor/dreamrefactory/node_modules/vitest/dist/config.js";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { restoredSaveFormat } from "./save-format-plugin";

const root = fileURLToPath(new URL(".", import.meta.url));
const vendor = resolve(root, "../../../vendor/dreamrefactory");
export default defineConfig({
  root,
  plugins: [restoredSaveFormat()],
  resolve: { alias: [
    { find: "vitest", replacement: `${vendor}/node_modules/vitest/dist/index.js` },
    { find: "@dreamfactory/engine/df/savegame", replacement: resolve(root, "save-format.ts") },
    { find: /^@dreamfactory\/engine\/(.*)$/, replacement: `${vendor}/engine/src/$1` },
    { find: /^@dreamfactory\/site\/(.*)$/, replacement: `${vendor}/site/src/$1` },
    { find: /^@dreamfactory\/taoot\/(.*)$/, replacement: `${vendor}/taoot/src/$1` },
  ] },
  test: { include: process.env.TITANIC_GAME_TESTS === "1" ? ["neutral-playthrough.test.ts"] : ["neutral-save.test.ts", "save-loading.test.ts", "save-dialogs.test.ts"], environment: "node", testTimeout: 120000, fileParallelism: false },
});
