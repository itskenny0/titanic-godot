import { test, expect } from "vitest";
import { GameSession } from "../../../vendor/dreamrefactory/engine/src/runtime/session";
import { NullAudioSink } from "../../../vendor/dreamrefactory/engine/src/runtime/audio";
import { parseSave } from "../../../vendor/dreamrefactory/engine/src/df/savegame";
import { createNeutralSaveTemplate, installRestoredSaving } from "./neutral-save";
import { parseRestoredSave } from "./save-extension";
import { readValidatedSave } from "./save-loading";

test("neutral envelope is deterministic, empty, and independent of game files or saves", () => {
  const bytes = createNeutralSaveTemplate();
  expect(bytes).toEqual(createNeutralSaveTemplate());
  const save = parseSave(bytes);
  expect(save.title).toBe("Titanic 1.0");
  expect(save.raw.containers).toHaveLength(12);
  expect(save.numGlobals.size + save.strGlobals.size).toBe(0);
  expect(save.inventory).toEqual([]);
  expect(save.actors).toEqual([]);
  expect(save.trackFiles).toEqual([]);
  expect(save.theme).toBeNull();
  expect(save.set + save.scene + save.view + save.disk).toBe("");
});

test("public snapshot keeps exact values and no inherited state without an original save", () => {
  const session = new GameSession(() => null, new NullAudioSink());
  installRestoredSaving(session);
  session.mountedCd = "Titanic2";
  session.currentSetFile = "example-room";
  session.frameCounter = 123456;
  session.interp.globals.set("mission", 2);
  session.interp.globals.set("precise", 2 ** 42 + .125);
  session.interp.globals.set("text", "A fresh voyage — 海");
  session.interp.globals.set("__internal", 1);
  for (let n = 0; n < 1000; n++) session.interp.globals.set(`authored-${n}`, `value-${n}`);
  const bytes = session.snapshotSave()!;
  const save = parseRestoredSave(bytes);
  expect(new Map([...save.numGlobals, ...save.strGlobals] as [string, number | string][])).toEqual(new Map([...session.interp.globals].filter(([name]) => !name.startsWith("__"))));
  expect(save.set).toBe("example-room");
  expect(save.disk).toBe("Titanic2");
  expect(save.frame).toBe(123456);
  expect(parseSave(bytes).numGlobals.size + parseSave(bytes).strGlobals.size).toBe(0);
  expect(readValidatedSave(Buffer.from(bytes).toString("base64"))).toEqual(bytes);
  const damaged = bytes.slice(); damaged[damaged.length - 22] ^= 1;
  expect(() => readValidatedSave(Buffer.from(damaged).toString("base64"))).toThrow("not a valid Titanic saved game");
});
