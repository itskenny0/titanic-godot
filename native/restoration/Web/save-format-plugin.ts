import type { Plugin } from "../../../vendor/dreamrefactory/node_modules/vite";
import { fileURLToPath } from "node:url";

const shim = fileURLToPath(new URL("./save-format.ts", import.meta.url));
/** Both load paths use the same decoder before the engine restores globals. */
export function restoredSaveFormat(): Plugin {
  return {
    name: "titanic-restored-save-format",
    enforce: "pre",
    resolveId(source, importer) {
      if (source === "../df/savegame" && importer?.replace(/\\/g, "/").endsWith("/engine/src/runtime/saveload.ts")) return shim;
      if (source === "@dreamfactory/engine/df/savegame") return shim;
      return null;
    },
  };
}
