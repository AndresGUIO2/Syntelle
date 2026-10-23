import { ActivityGate, PCMResampler } from './resampler'
import { inputLevel } from './inputLevel'

declare const sampleRate: number
declare function registerProcessor(name: string, processor: typeof AudioWorkletProcessor): void
declare abstract class AudioWorkletProcessor {
  readonly port: MessagePort
  abstract process(inputs: Float32Array[][]): boolean
}

class ConversationCapture extends AudioWorkletProcessor {
  private readonly resampler = new PCMResampler(sampleRate)
  private readonly gate = new ActivityGate()
  private packet: number[] = []
  private pending = 0
  constructor() {
    super()
    this.port.onmessage = () => {
      this.pending = Math.max(0, this.pending - 1)
    }
  }
  process(inputs: Float32Array[][]) {
    const channel = inputs[0]?.[0]
    if (!channel) return true
    const pcm = this.resampler.push(channel)
    if (this.gate.update(pcm)) this.port.postMessage({ type: 'activity' })
    for (const sample of pcm) this.packet.push(sample)
    if (this.packet.length >= 1600) {
      if (this.pending >= 20) {
        this.port.postMessage({ type: 'overflow' })
        return false
      }
      // Explicit little-endian PCM, regardless of host byte order.
      const bytes = new ArrayBuffer(this.packet.length * 2)
      const view = new DataView(bytes)
      this.packet.forEach((value, i) => view.setInt16(i * 2, value, true))
      const level = inputLevel(this.packet)
      this.packet = []
      this.port.postMessage({ type: 'pcm', bytes, level }, [bytes])
      this.pending++
    }
    return true
  }
}
registerProcessor('syntelle-conversation-capture', ConversationCapture)
