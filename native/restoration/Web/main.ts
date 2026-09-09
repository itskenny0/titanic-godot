import { DeferredAudioSink, WebAudioSink } from "@dreamfactory/engine/runtime/audio";
import { GameHost } from "@dreamfactory/engine/web/host";
import { CursorSheet } from "@dreamfactory/engine/web/cursors";
import { appendSaveMetadata } from "./save-extension";
import { guardLoadDialog, guardSaveDialog } from "./save-dialogs";
import { applyPendingSave, InvalidSaveError, readValidatedSave } from "./save-loading";
import { installRestoredSaving } from "./neutral-save";
import { FileStore } from "@dreamfactory/taoot/files";
import { TI_CURSORS } from "@dreamfactory/taoot/cursor-art";
import { ALL_CHANNELS, resetScreenGamma, stepScreenGamma } from "@dreamfactory/engine/web/screen-gamma";

const ENGINE_REVISION = "b43a02668f3db36519bd5b44a5892fdefd292208";
type Reply = { id: string; ok: boolean; result?: unknown; error?: string };
type Command = { action: string };
declare global {
  interface Window {
    webkit?: { messageHandlers: { titanicHost: { postMessage(value: unknown): void } } };
    titanicHost: { receive(reply: Reply): void; onCommand(command: Command): void };
  }
}
let nextRequest = 0;
const waiting = new Map<string, { resolve(value: any): void; reject(error: Error): void }>();
function native<T = any>(action: string, fields: Record<string, unknown> = {}): Promise<T> {
  const id = String(++nextRequest);
  return new Promise((resolve, reject) => {
    if (!window.webkit?.messageHandlers.titanicHost) return reject(new Error("The game host is unavailable."));
    waiting.set(id, { resolve, reject });
    window.webkit.messageHandlers.titanicHost.postMessage({ id, action, ...fields });
  });
}
function announce(action: string, fields: Record<string, unknown>): void {
  window.webkit?.messageHandlers.titanicHost.postMessage({ action, ...fields });
}
window.titanicHost = {
  receive(reply) {
    const pending = waiting.get(reply.id);
    if (!pending) return;
    waiting.delete(reply.id);
    if (reply.ok) pending.resolve(reply.result);
    else pending.reject(new Error(reply.error || "The operation could not be completed."));
  },
  onCommand(command) { void performCommand(command.action).catch(reportError); },
};

const screen = document.querySelector<HTMLCanvasElement>("#screen")!;
const ctx = screen.getContext("2d", { alpha: false })!;
const status = document.querySelector<HTMLDivElement>("#status")!;
const statusText = document.querySelector<HTMLParagraphElement>("#statusText")!;
const pausedOverlay = document.querySelector<HTMLDivElement>("#paused")!;
const files = new FileStore();
const audio = new DeferredAudioSink();
const audioContext = new AudioContext();
audio.attach(new WebAudioSink(audioContext));
const recentErrors: string[] = [];
const legacySaveWarnings: string[] = [];
let lastRestoredSave: { bytes: number; globals: number } | null = null;
let frameCount = 0;
let lastFrame = 0;
let slowFrames = 0;
let reportedFrame = 0;
let reportedTime = performance.now();
let lastInput: Record<string, unknown> | null = null;
let pauseOwned = false;
let paused = false;
let menuBusy = false;
let openedSaveQueued = false;
let bootReady = false;
const cursors = new CursorSheet(TI_CURSORS);
let cursorName = "arrow";

function log(message: string): void {
  if (message.includes("could not be carried")) {
    // The unextended Windows skeleton has fixed capacity. Our trailer is the
    // authoritative complete state; preserve this useful compatibility detail
    // without misreporting a successful restored save as lost game progress.
    legacySaveWarnings.push(message);
    if (legacySaveWarnings.length > 20) legacySaveWarnings.shift();
  } else if (/error|failed|not available/i.test(message)) {
    recentErrors.push(message);
    if (recentErrors.length > 20) recentErrors.shift();
  }
  announce("log", { message });
}
function reportError(error: unknown): void {
  void presentError(error);
}
async function presentError(error: unknown): Promise<void> {
  const message = error instanceof Error ? error.message : String(error);
  const detail = error instanceof InvalidSaveError ? ` (${String(error.cause)})` : "";
  log(`error: ${message}${detail}`);
  await native("noteDialog", { title: "Titanic", message }).catch(() => {});
}
const host = new GameHost(files, audio, {
  log,
  hud: (message) => announce("log", { message, event: "room" }),
  showStage: () => { status.hidden = true; screen.focus(); },
});
const session = host.session;
// Use each original high-resolution standpoint immediately after movement,
// avoiding the old right-turn blur/sharpen flash without replacing any art.
session.pictureMode = "sharp";
session.hasRealFrames = true;
session.nextFrame = () => new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
installRestoredSaving(session);
session.onNoteDialog = async (message) => { await withFrozen(() => native("noteDialog", { title: "Titanic", message })); };
session.onQuestionDialog = async (message) => withFrozen(async () => (await native("questionDialog", { title: "Titanic", message })).accepted);
session.onTextDialog = async (message, initial) => withFrozen(async () => (await native("textDialog", { title: "Titanic", message, defaultValue: initial })).text ?? "");
session.onQuit = () => { announce("quit", {}); };
session.onSaveGame = async (bytes) => guardSaveDialog(() => saveBytes(bytes), presentError);
session.onLoadGame = async () => chooseSave();
host.director.onCursor = showCursor;

function showCursor(name: string): void {
  cursorName = name || "arrow";
  const system = ({ touch: "pointer", hand: "grab", fist: "grabbing", watch: "wait", godown: "s-resize", goleft: "w-resize", goright: "e-resize", gostrait: "n-resize", goup: "n-resize", none: "none" } as Record<string, string>)[cursorName.toLowerCase()] ?? "default";
  screen.style.cursor = Object.keys(TI_CURSORS).length ? cursors.css(cursorName, screen.getBoundingClientRect().width / screen.width) : system;
}
window.addEventListener("resize", () => showCursor(cursorName));
function encode(bytes: Uint8Array): string {
  let result = "";
  for (let i = 0; i < bytes.length; i += 8192) result += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(result);
}
async function chooseSave(): Promise<Uint8Array | null> {
  return guardLoadDialog(async () => {
    const result = await native<{ name: string; data: string } | null>("chooseSave");
    if (!result) return null;
    const bytes = readValidatedSave(result.data);
    log(`loading saved game: ${result.name}`);
    return bytes;
  }, presentError);
}
async function saveBytes(bytes: Uint8Array): Promise<void> {
  bytes = appendSaveMetadata(bytes, session);
  const savedGlobals = [...session.interp.globals.keys()].filter((name) => !name.startsWith("__")).length;
  const suggested = `${session.currentSetFile || "Voyage"} ${new Date().toLocaleString("en-CA", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).replace(/[:,]/g, "-")}`;
  const answer = await native<{ text: string | null }>("textDialog", { title: "Save Game", message: "Name this saved game.", defaultValue: suggested });
  if (!answer?.text?.trim()) return;
  const name = answer.text.trim().replace(/\.ti$/i, "") + ".ti";
  const existing = await native<{ name: string }[]>("listSaves");
  const duplicate = existing.some((save) => save.name.toLowerCase() === name.toLowerCase());
  if (duplicate && !(await native("questionDialog", { title: "Replace Saved Game?", message: `Replace “${name}” with your current progress?`, buttons: ["Replace", "Cancel"] })).accepted) return;
  await native("writeSave", { name, data: encode(bytes), overwrite: duplicate });
  lastRestoredSave = { bytes: bytes.length, globals: savedGlobals };
  log(`saved game: ${name} (${bytes.length} bytes)`);
}
async function withFrozen<T>(work: () => Promise<T>): Promise<T> {
  const owns = !session.frozen;
  if (owns) session.freezeTime();
  try { return await work(); } finally { if (owns) session.thawTime(); resumeAudio(); }
}
let lastAudioResume = 0;
function resumeAudio(): void {
  if (paused || session.frozen || document.hidden || audioContext.state === "running" || audioContext.state === "closed") return;
  const now = performance.now();
  if (now - lastAudioResume < 500) return;
  lastAudioResume = now;
  void audioContext.resume().catch((error) => log(`audio resume failed: ${String(error)}`));
}
window.addEventListener("focus", resumeAudio);
window.addEventListener("pageshow", resumeAudio);
document.addEventListener("visibilitychange", resumeAudio);
async function performCommand(action: string): Promise<void> {
  // Finder may deliver a document while the bundle is loading or another
  // native panel is up. Leave its bytes in the host until we can validate them.
  if (action === "loadOpenedSave") openedSaveQueued = true;
  if (action === "resumeAudio") { resumeAudio(); return; }
  if (action === "pause") {
    if (paused) return;
    paused = true;
    pauseOwned = !session.frozen;
    if (pauseOwned) session.freezeTime();
    pausedOverlay.hidden = false;
    session.pointerDown = false;
    return;
  }
  if (action === "resume") {
    paused = false;
    if (pauseOwned) session.thawTime();
    pauseOwned = false;
    pausedOverlay.hidden = true;
    screen.focus();
    resumeAudio();
    return;
  }
  if (!bootReady || menuBusy) return;
  menuBusy = true;
  try {
    if (action === "loadOpenedSave") {
      openedSaveQueued = false;
      await withFrozen(() => applyPendingSave({
        take: () => native("takePendingLoad"),
        load: async (bytes) => { await native("setPendingLoad", { data: encode(bytes) }); location.reload(); },
        fallback: async () => {},
        report: presentError,
      }));
    } else if (action === "save") {
      if (!session.currentSetFile || host.director.inputLocked || !session.viewShowing) {
        await native("noteDialog", { title: "Save Game", message: "Finish the current conversation or animation, then save while exploring. You can also save from the game’s life preserver menu." });
        return;
      }
      await withFrozen(async () => {
        const bytes = session.snapshotSave();
        if (!bytes) throw new Error("The current game could not be saved.");
        await saveBytes(bytes);
      });
    } else if (action === "load") {
      const bytes = await withFrozen(chooseSave);
      if (bytes) {
        await native("setPendingLoad", { data: encode(bytes) });
        location.reload();
      }
    } else if (action === "newGame") {
      const ok = await withFrozen(async () => (await native("questionDialog", { title: "Return to the Main Menu?", message: "Unsaved progress in this voyage will be lost.", buttons: ["Main Menu", "Cancel"] })).accepted);
      if (ok) location.reload();
    }
  } finally {
    menuBusy = false;
    if (openedSaveQueued) void performCommand("loadOpenedSave").catch(reportError);
  }
}
function point(e: PointerEvent | MouseEvent): { x: number; y: number } {
  const rect = screen.getBoundingClientRect();
  return { x: Math.floor((e.clientX - rect.left) / rect.width * screen.width), y: Math.floor((e.clientY - rect.top) / rect.height * screen.height) };
}
screen.addEventListener("pointerdown", (event) => {
  if (paused || menuBusy || event.button !== 0) return;
  resumeAudio();
  screen.focus();
  const { x, y } = point(event);
  session.setPointer(x, y);
  session.pointerDown = true;
  screen.setPointerCapture(event.pointerId);
  session.shiftDown = event.shiftKey;
  lastInput = { kind: "press", x, y, at: Date.now() };
  void session.track(host.director.press(x, y), `press ${x},${y}`).catch(reportError);
  event.preventDefault();
});
window.addEventListener("pointerup", (event) => {
  const { x, y } = point(event);
  session.setPointer(x, y);
  session.pointerDown = false;
  host.director.release(x, y);
});
window.addEventListener("pointercancel", () => { session.pointerDown = false; host.director.release(-1, -1); });
screen.addEventListener("pointermove", (event) => {
  if (paused || menuBusy) return;
  const { x, y } = point(event);
  if (session.pointerDown) session.setPointer(x, y);
  else void host.director.hover(x, y).then(showCursor).catch(reportError);
});
window.addEventListener("keydown", (event) => {
  if (paused || menuBusy || event.metaKey || event.altKey) return;
  resumeAudio();
  const gamma = /^F([1-9])$/.exec(event.key);
  if (gamma) {
    const key = Number(gamma[1]);
    if (key === 9) resetScreenGamma();
    else stepScreenGamma(key % 2 === 0, key < 3 ? ALL_CHANNELS : [key < 5, key >= 5 && key < 7, key >= 7]);
    event.preventDefault();
    return;
  }
  const name = ({ ArrowUp: "uparrow", ArrowDown: "downarrow", ArrowLeft: "leftarrow", ArrowRight: "rightarrow", Escape: ".", Enter: "\r", Backspace: "\b", Tab: "\t" } as Record<string,string>)[event.key] ?? (event.key.length === 1 ? event.key.toLowerCase() : "");
  if (!name) return;
  lastInput = { kind: "key", key: event.key, at: Date.now() };
  const arrow = name === "uparrow" || name === "leftarrow" || name === "rightarrow";
  const work = arrow && host.viewer && (session.viewShowing || !session.stageCtrl.keydownTarget())
    ? host.viewer.pressNav(name)
    : host.director.keyDown(name, event.key === "Escape" || event.ctrlKey);
  void session.track<unknown>(work, `key ${name}`).catch(reportError);
  event.preventDefault();
});
window.addEventListener("error", (event) => log(`uncaught error: ${event.message}`));
window.addEventListener("unhandledrejection", (event) => log(`unhandled rejection: ${String(event.reason)}`));

function frame(now: number): void {
  if (lastFrame && now - lastFrame > 80 && !paused) slowFrames++;
  lastFrame = now;
  host.director.tick(now);
  host.director.render(ctx);
  frameCount++;
  requestAnimationFrame(frame);
}
requestAnimationFrame(frame);
setInterval(() => {
  resumeAudio();
  const now = performance.now();
  const fps = (frameCount - reportedFrame) * 1000 / (now - reportedTime);
  reportedFrame = frameCount;
  reportedTime = now;
  announce("diagnostics", {
    engineRevision: ENGINE_REVISION, frameCount, slowFrames, fps, audioState: audioContext.state,
    audioChannels: { sound: !audio.isDone("sound"), voice: !audio.isDone("voice"), theme: !audio.isDone("theme") },
    audioTime: audioContext.currentTime, disc: files.activeDisc(), set: session.currentSetFile,
    scene: session.currentSceneName(), view: session.currentViewName(), theme: session.currentThemeName,
    mission: session.interp.globals.get("mission"), phase: session.interp.globals.get("phase"),
    frozen: session.frozen, paused, inputLocked: host.director.inputLocked, moviePlaying: host.director.movies.playing,
    frame: session.frameCounter, lastInput, errors: recentErrors, legacySaveWarnings, lastRestoredSave,
  });
}, 2000);
async function boot(): Promise<void> {
  const response = await fetch("./gamefiles.json");
  if (!response.ok) throw new Error("The bundled game data could not be opened.");
  const manifest = await response.json() as Record<string, number>;
  for (const path of Object.keys(manifest)) {
    files.registerServerFile(path.split("/").pop()!, new URL(path.split("/").map(encodeURIComponent).join("/"), location.href).href);
  }
  files.setEdition("en");
  const { volumes } = await host.bootPlan();
  files.setVolumes(volumes);
  await host.preload({ onProgress: (progress) => { statusText.textContent = "Preparing your voyage…"; } });
  status.hidden = true;
  screen.focus();
  showCursor("arrow");
  const ready = (): void => {
    bootReady = true;
    announce("ready", { details: { engineRevision: ENGINE_REVISION, files: Object.keys(manifest).length, volumes } });
    resumeAudio();
    // Also drain once when becoming ready: Finder can arrive after the initial
    // take reply but before native sees our ready announcement. The host keeps
    // that document pending even though it could not send a command yet.
    queueMicrotask(() => { void performCommand("loadOpenedSave").catch(reportError); });
  };
  openedSaveQueued = false;
  await applyPendingSave({
    take: () => native("takePendingLoad"),
    load: async (bytes) => { ready(); await host.loadSavedGame(bytes); },
    fallback: async () => { ready(); await session.track(host.coldBoot(), "coldBoot"); },
    report: presentError,
  });
}
void boot().catch((error) => {
  status.hidden = false;
  statusText.textContent = error instanceof Error ? error.message : String(error);
  log(`boot failed: ${statusText.textContent}`);
});
