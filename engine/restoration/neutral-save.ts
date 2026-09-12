import { writeSaveFile } from "../../vendor/dreamrefactory/engine/src/df/savegame";
import type { GameSession } from "../../vendor/dreamrefactory/engine/src/runtime/session";
import { appendSaveMetadata } from "./save-extension";

/** An authored, empty format envelope. Every byte starts at zero and only
 * documented structural constants/type identifiers are written. No original
 * game file, save, palette, recording, or process memory supplies these bytes.
 * New Mac saves store their authoritative state in our versioned extension;
 * this envelope is not intended for the original Windows executable. */
export function createNeutralSaveTemplate(): Uint8Array {
  const header = new Uint8Array(1024);
  new DataView(header.buffer).setUint32(0, 0x00010000, true);
  header.set(new TextEncoder().encode("ODTRTRFD"), 32);
  const sizes = [272, 786, 0, 0, 0, 0, 0, 28, 0, 32 * 42, 16 * 74, 16 * 110];
  const containers = sizes.map((size, id) => ({ id, data: new Uint8Array(size) }));
  const pstr = (data: Uint8Array, offset: number, text: string): void => {
    const value = new TextEncoder().encode(text);
    data[offset] = value.length;
    data.set(value, offset + 1);
  };
  // The engine's file-version discriminator, not a copied save header.
  pstr(containers[0].data, 0, "Titanic 1.0");
  pstr(containers[1].data, 520, "main.stg");
  return writeSaveFile({ header, table: new Uint8Array(512), containers });
}

export function createRestoredSnapshot(session: GameSession): Uint8Array {
  return appendSaveMetadata(createNeutralSaveTemplate(), session);
}

export function installRestoredSaving(session: GameSession): void {
  session.saveTemplate = createNeutralSaveTemplate;
  session.snapshotSave = () => createRestoredSnapshot(session);
}
