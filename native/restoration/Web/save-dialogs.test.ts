import { test, expect } from "vitest";
import { GameSession } from "../../../vendor/dreamrefactory/engine/src/runtime/session";
import { NullAudioSink } from "../../../vendor/dreamrefactory/engine/src/runtime/audio";
import { installRestoredSaving } from "./neutral-save";
import { guardLoadDialog, guardSaveDialog } from "./save-dialogs";
import type { Frame } from "../../../vendor/dreamrefactory/engine/src/runtime/interp";

test("original load builtin restores the screen after read, decoding, or validation failure", async () => {
  const p = { session: new GameSession(() => null, new NullAudioSink()) };
  installRestoredSaving(p.session);
  for (const failure of ["The selected file could not be read.", "Invalid base64 data.", "This restored save is incomplete or damaged."]) {
    const notices: string[] = [];
    p.session.fade.level = 0.25;
    p.session.onLoadGame = () => guardLoadDialog(async () => { throw new Error(failure); }, async (error) => {
      expect(p.session.fade.level).toBe(1);
      expect(p.session.frozen).toBe(true);
      await Promise.resolve();
      notices.push((error as Error).message);
    });
    const builtin = p.session.interp.builtins.get("opengame")!;
    await builtin(p.session.interp, [], {} as never, {} as Frame);
    expect(notices).toEqual([failure]);
    expect(p.session.fade.level).toBe(0.25);
    expect(p.session.frozen).toBe(false);
  }
});

test("original save builtin returns to CTL script only after a failed write's notice closes", async () => {
  const p = { session: new GameSession(() => null, new NullAudioSink()) };
  installRestoredSaving(p.session);
  const notices: string[] = [];
  p.session.onSaveGame = () => guardSaveDialog(async () => { throw new Error("Use a name without slashes."); }, async (error) => {
    expect(p.session.frozen).toBe(true);
    await Promise.resolve();
    notices.push((error as Error).message);
  });
  const builtin = p.session.interp.builtins.get("savegame")!;
  await builtin(p.session.interp, [], {} as never, {} as Frame);
  expect(notices).toEqual(["Use a name without slashes."]);
  expect(p.session.frozen).toBe(false);
});
