/** Original CTL.STG must regain control after an OS dialog fails, so it can
 * uncover the room. Resolve a failed load as cancellation, and a failed save
 * only after its error notice closes. */
export async function guardLoadDialog(load: () => Promise<Uint8Array | null>, report: (error: unknown) => Promise<void>): Promise<Uint8Array | null> {
  try { return await load(); } catch (error) { await report(error); return null; }
}
export async function guardSaveDialog(save: () => Promise<void>, report: (error: unknown) => Promise<void>): Promise<void> {
  try { await save(); } catch (error) { await report(error); }
}
