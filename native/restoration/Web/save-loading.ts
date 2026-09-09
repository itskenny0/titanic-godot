import { parseSave } from "./save-format";

export class InvalidSaveError extends Error {
  constructor(cause: unknown) {
    super("This file is not a valid Titanic saved game, or it is damaged. Choose another saved game.", { cause });
    this.name = "InvalidSaveError";
  }
}

/** Validate before replacing the running page; the upstream host deliberately
 * reports parse errors only to its HUD and cannot provide this guarantee. */
export function readValidatedSave(data: string): Uint8Array {
  try {
    const bytes = Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
    parseSave(bytes);
    return bytes;
  } catch (cause) { throw new InvalidSaveError(cause); }
}

export async function applyPendingSave(options: {
  take(): Promise<{ data: string } | null>;
  load(bytes: Uint8Array): Promise<void>;
  fallback(): Promise<void>;
  report(error: unknown): Promise<void>;
}): Promise<void> {
  let bytes: Uint8Array | null = null;
  try {
    const pending = await options.take();
    if (pending) bytes = readValidatedSave(pending.data);
  } catch (error) { await options.report(error); }
  if (bytes) await options.load(bytes);
  else await options.fallback();
}
