// Keep the upstream parser intact for original 1996 saves, and let the restored
// app replace its decoded state with the complete checked metadata when present.
export * from "../../vendor/dreamrefactory/engine/src/df/savegame";
export { parseRestoredSave as parseSave } from "./save-extension";
