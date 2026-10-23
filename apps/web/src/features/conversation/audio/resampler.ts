// Streaming windowed-sinc resampling: preserve phase and filter history across
// AudioWorklet blocks. A 7.2 kHz cutoff suppresses aliases at 16 kHz output.
export class PCMResampler {
  private samples: number[] = []
  private position = 24
  private readonly ratio: number
  private readonly cutoff: number
  constructor(
    sourceRate: number,
    readonly targetRate = 16000,
  ) {
    if (!Number.isFinite(sourceRate) || sourceRate < 16000 || sourceRate > 192000)
      throw Error('Unsupported microphone sample rate.')
    this.ratio = sourceRate / targetRate
    this.cutoff = Math.min(0.45, (targetRate * 0.45) / sourceRate)
    this.samples = Array(24).fill(0)
  }
  push(input: Float32Array): Int16Array {
    for (const sample of input) this.samples.push(sample)
    const output: number[] = []
    while (this.position + 24 < this.samples.length) {
      const center = Math.floor(this.position)
      let sum = 0,
        weight = 0
      for (let i = center - 23; i <= center + 24; i++) {
        const distance = i - this.position
        const x = 2 * this.cutoff * distance
        const sinc = Math.abs(x) < 1e-8 ? 1 : Math.sin(Math.PI * x) / (Math.PI * x)
        const window = 0.5 + 0.5 * Math.cos((Math.PI * distance) / 24)
        const coefficient = 2 * this.cutoff * sinc * window
        sum += (this.samples[i] ?? 0) * coefficient
        weight += coefficient
      }
      output.push(Math.round(Math.max(-1, Math.min(1, sum / weight)) * 32767))
      this.position += this.ratio
    }
    const discard = Math.max(0, Math.floor(this.position) - 24)
    this.samples.splice(0, discard)
    this.position -= discard
    return Int16Array.from(output)
  }
}

export class ActivityGate {
  private noise = 0.003
  private activeSamples = 0
  private quietSamples = 0
  private latched = false
  update(pcm: Int16Array): boolean {
    let square = 0
    for (const sample of pcm) square += (sample / 32768) ** 2
    const rms = Math.sqrt(square / Math.max(1, pcm.length))
    const threshold = Math.max(0.015, this.noise * 3)
    if (rms >= threshold) {
      this.activeSamples += pcm.length
      this.quietSamples = 0
    } else {
      this.noise = this.noise * 0.99 + rms * 0.01
      this.activeSamples = 0
      this.quietSamples += pcm.length
      if (this.quietSamples >= 5600) this.latched = false
    }
    if (!this.latched && this.activeSamples >= 1920) {
      this.latched = true
      return true
    }
    return false
  }
}
