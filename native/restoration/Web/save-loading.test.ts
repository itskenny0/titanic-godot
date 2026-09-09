import { test, expect } from "vitest";
import { applyPendingSave, InvalidSaveError, readValidatedSave } from "./save-loading";
import { createNeutralSaveTemplate } from "./neutral-save";

const valid = Buffer.from(createNeutralSaveTemplate());

test("Finder rejects damaged saves before any pending write or reload, preserving the running game", async () => {
  for (const data of ["!not base64!", Buffer.from("not a Titanic save").toString("base64")]) {
    const events: string[] = [];
    await applyPendingSave({
      take: async () => ({ data }),
      load: async () => { events.push("setPendingLoad", "reload"); },
      fallback: async () => { events.push("keep-playing"); },
      report: async (error) => {
        expect(error).toBeInstanceOf(InvalidSaveError);
        expect((error as Error).message).toContain("not a valid Titanic saved game");
        expect((error as Error).cause).toBeDefined();
        events.push("notice");
      },
    });
    expect(events).toEqual(["notice", "keep-playing"]);
  }
});

test("a bad startup document shows its notice and then boots the original main menu", async () => {
  const events: string[] = [];
  await applyPendingSave({
    take: async () => ({ data: "AAAA" }),
    load: async () => { events.push("load"); },
    fallback: async () => { events.push("coldBoot"); },
    report: async () => { await Promise.resolve(); events.push("notice-closed"); },
  });
  expect(events).toEqual(["notice-closed", "coldBoot"]);
});

test("a valid pending save reaches reload only after validation", async () => {
  const data = valid.toString("base64");
  expect(readValidatedSave(data)).toEqual(new Uint8Array(valid));
  const events: string[] = [];
  await applyPendingSave({
    take: async () => ({ data }),
    load: async (bytes) => { expect(bytes).toEqual(new Uint8Array(valid)); events.push("setPendingLoad", "reload"); },
    fallback: async () => { events.push("coldBoot"); },
    report: async () => { events.push("notice"); },
  });
  expect(events).toEqual(["setPendingLoad", "reload"]);
});
