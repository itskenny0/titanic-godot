// Restoration-owned, versioned save data. The original .ti remains the base;
// the trailer carries values which its fixed Windows heap cannot represent.
import { parseSave as parseOriginalSave, type SaveGame } from "../../../vendor/dreamrefactory/engine/src/df/savegame";
import type { GameSession } from "../../../vendor/dreamrefactory/engine/src/runtime/session";

const MARK = new TextEncoder().encode("TITANIC-MAC-SAVE\n");
const LIMIT = 4 * 1024 * 1024;
const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });
type Metadata = Pick<SaveGame, "disk" | "set" | "scene" | "view" | "frame" | "inventory" | "actors" | "loops" | "crickets" | "walks" | "theme" | "castFiles" | "trackFiles"> & {
  version: 1;
  globals: [string, string | number][];
};
const fail = (): never => { throw new Error("This restored save is incomplete or damaged. Your current game has not been replaced."); };
function crc32(bytes: Uint8Array): number {
  let crc = 0xffffffff;
  for (const byte of bytes) {
    crc ^= byte;
    for (let i = 0; i < 8; i++) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
  }
  return (crc ^ 0xffffffff) >>> 0;
}
function marked(bytes: Uint8Array, offset: number): boolean {
  return offset >= 0 && offset + MARK.length <= bytes.length && MARK.every((b, i) => bytes[offset + i] === b);
}
function split(bytes: Uint8Array): { raw: Uint8Array; metadata?: Metadata } {
  if (bytes.length < 8) return { raw: bytes };
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const rawSize = dv.getUint32(4, true);
  const atStart = marked(bytes, rawSize);
  const atEnd = marked(bytes, bytes.length - MARK.length);
  const remainder = bytes.length - rawSize;
  if (rawSize >= 1536 && remainder > 0 && remainder < MARK.length &&
      bytes.subarray(rawSize).every((b, i) => b === MARK[i])) fail();
  if (!atStart && !atEnd) return { raw: bytes };
  if (!atStart || !atEnd || rawSize < 1536 || bytes.length > rawSize + LIMIT + MARK.length * 2 + 8) fail();
  const footer = bytes.length - MARK.length - 8;
  if (footer < rawSize + MARK.length) fail();
  const length = dv.getUint32(footer, true);
  if (length > LIMIT || rawSize + MARK.length + length !== footer) fail();
  if (crc32(bytes.subarray(0, footer)) !== dv.getUint32(footer + 4, true)) fail();
  let value: unknown;
  try { value = JSON.parse(decoder.decode(bytes.subarray(rawSize + MARK.length, footer))); } catch { fail(); }
  validateMetadata(value);
  return { raw: bytes.slice(0, rawSize), metadata: value };
}

function object(value: unknown): asserts value is Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) fail();
}
function str(value: unknown): asserts value is string { if (typeof value !== "string" || value.length > 65536) fail(); }
function num(value: unknown): asserts value is number { if (typeof value !== "number" || !Number.isFinite(value)) fail(); }
function bool(value: unknown): void { if (typeof value !== "boolean") fail(); }
function fields(value: unknown, strings: string[], numbers: string[], booleans: string[] = []): asserts value is Record<string, any> {
  object(value);
  for (const key of strings) str(value[key]);
  for (const key of numbers) num(value[key]);
  for (const key of booleans) bool(value[key]);
}
function array(value: unknown, check: (entry: any) => void): void {
  if (!Array.isArray(value) || value.length > 10000) return fail();
  for (const entry of value) check(entry);
}
function validateMetadata(value: unknown): asserts value is Metadata {
  fields(value, ["disk", "set", "scene", "view"], ["frame"]);
  if (value.version !== 1 || value.frame < 0) fail();
  const names = new Set<string>();
  array(value.globals, (entry) => {
    if (!Array.isArray(entry) || entry.length !== 2) fail();
    str(entry[0]);
    if (!entry[0] || entry[0].length > 512 || entry[0].startsWith("__") || names.has(entry[0].toLowerCase())) fail();
    names.add(entry[0].toLowerCase());
    if (typeof entry[1] === "string") str(entry[1]); else num(entry[1]);
  });
  array(value.inventory, (p) => fields(p, ["name", "view", "owner"], ["x", "y", "deg", "dist", "scale", "value", "zclip"], ["visible", "is3d"]));
  array(value.actors, (a) => {
    fields(a, ["name", "owner"], ["value"]);
    fields(a.placement, ["set", "star", "pose"], ["x", "y", "z", "deg", "speed", "turn", "scale", "zclip"], ["visible"]);
  });
  array(value.loops, (l) => {
    fields(l, ["kind", "name", "handler"], ["period"]);
    if (!["actor", "prop", "scene", "flat"].includes(l.kind)) fail();
  });
  array(value.crickets, (c) => fields(c, ["name", "set"], ["x", "y", "radius", "base", "jitter", "next"]));
  array(value.walks, (w) => {
    fields(w, ["actor", "star"], ["type", "turnTo", "deg", "startX", "startY", "startZ", "destX", "destY", "destZ", "progress", "dist"], ["hasPayload", "paused"]);
    if (![0, 1, 3].includes(w.type)) fail();
    if (w.path !== undefined) array(w.path, (p) => fields(p, [], ["x", "y", "z", "cum"]));
    if (w.type === 3 && (!Array.isArray(w.path) || w.path.length < 2)) fail();
  });
  array(value.castFiles, str);
  array(value.trackFiles, str);
  if (value.theme !== null) fields(value.theme, ["track"], ["volume", "extras"]);
}

export function appendSaveMetadata(rawBytes: Uint8Array, session: GameSession): Uint8Array {
  const { raw } = split(rawBytes);
  parseOriginalSave(raw); // never wrap an invalid base
  const metadata: Metadata = {
    version: 1,
    globals: [...session.interp.globals].filter(([name]) => !name.startsWith("__")),
    disk: session.mountedCd,
    set: session.currentSetFile,
    scene: session.currentSceneName(),
    view: session.currentViewName(),
    frame: session.frameCounter,
    inventory: [...session.propRuntime.props].map(([name, p]) => ({
      name, owner: String(p.owner) || "none", view: String(p.stateName ?? ""),
      visible: !!p.visible, is3d: p.worldSpace, x: p.anchorX, y: p.anchorY,
      deg: Number(p.deg), dist: p.dist, scale: p.scale, value: Number(p.value), zclip: p.zclip,
    })),
    actors: [...session.actorRuntime.actors].map(([name, a]) => ({
      name, owner: String(a.owner) || "none", value: Number(a.value),
      placement: { visible: !!a.visible, set: a.setName, star: a.starName, pose: a.poseName,
        x: a.worldX, y: a.worldY, z: a.worldZ, deg: a.deg, speed: a.speed, turn: a.turn, scale: a.scale, zclip: a.zclip },
    })),
    loops: session.scheduler.loops.map((l) => ({ kind: l.kind, name: l.name, handler: l.handler, period: l.count })),
    crickets: session.scheduler.crickets.map((c) => ({ name: c.name, set: c.setName, x: c.x, y: c.y, radius: c.radius, base: c.base, jitter: c.jitter, next: c.count })),
    walks: [...session.scheduler.walks].map(([name, w]) => {
      const a = session.actorRuntime.get(name);
      const path = w.path?.map((p) => ({ ...p }));
      const type = w.turnOnly ? 0 : path && path.length > 1 ? 3 : 1;
      return { actor: name, type, hasPayload: type === 3, paused: w.paused, turnTo: w.turnTo ?? -1,
        deg: a?.deg ?? 0, startX: w.sx, startY: w.sy, startZ: w.sz,
        destX: w.sx + w.dx, destY: w.sy + w.dy, destZ: w.sz + w.dz,
        progress: w.progress, dist: w.dist, star: w.arriveStar ?? (w.turnOnly ? a?.starName ?? "" : ""),
        ...(path ? { path } : {}) };
    }),
    theme: session.currentThemeName === "none" ? null : {
      track: session.currentThemeName, volume: Number(session.interp.globals.get("themevolume") ?? 255), extras: 0,
    },
    castFiles: [...session.actorRuntime.casts.keys()],
    trackFiles: session.audioLib.bankNames,
  };
  validateMetadata(metadata);
  const payload = encoder.encode(JSON.stringify(metadata));
  if (payload.length > LIMIT) throw new Error("This saved game is too large to store safely.");
  const footer = raw.length + MARK.length + payload.length;
  const result = new Uint8Array(footer + 8 + MARK.length);
  result.set(raw); result.set(MARK, raw.length); result.set(payload, raw.length + MARK.length);
  const dv = new DataView(result.buffer);
  dv.setUint32(footer, payload.length, true); dv.setUint32(footer + 4, crc32(result.subarray(0, footer)), true);
  result.set(MARK, footer + 8);
  return result;
}

/** Installed through the restoration's resolver shim at both host and runtime
 * parse sites, so exact values exist BEFORE the departing session is discarded. */
export function parseRestoredSave(bytes: Uint8Array): SaveGame {
  const { raw, metadata } = split(bytes);
  const save = parseOriginalSave(raw);
  if (!metadata) return save;
  const { version: _version, globals, ...state } = metadata;
  Object.assign(save, state);
  save.numGlobals = new Map(globals.filter(([, v]) => typeof v === "number") as [string, number][]);
  save.strGlobals = new Map(globals.filter(([, v]) => typeof v === "string") as [string, string][]);
  save.hallside = save.strGlobals.get("hallside") ?? "";
  save.savedeck = save.strGlobals.get("savedeck") ?? "";
  save.clock = String(save.strGlobals.get("clock") ?? save.numGlobals.get("clock") ?? "");
  return save;
}
