/** Visual-only PCM16 RMS meter. Recognition and activity detection do not use it. */
export function inputLevel(samples: readonly number[]): number {
  if (!samples.length) return 0
  let squares = 0
  for (const sample of samples) squares += (sample / 32768) ** 2
  const rms = Math.sqrt(squares / samples.length)
  const db = 20 * Math.log10(Math.max(rms, 1e-8))
  return Math.min(1, Math.max(0, (db + 48) / 30))
}
