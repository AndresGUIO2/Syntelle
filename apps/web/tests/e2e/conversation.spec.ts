import { expect, test, type Page } from '@playwright/test'
import { inputLevel } from '../../src/features/conversation/audio/inputLevel'
import { PCMResampler, ActivityGate } from '../../src/features/conversation/audio/resampler'

function preparedAudio(duration = 3000) {
  const rate = 24000,
    count = (rate * duration) / 1000,
    bytes = Buffer.alloc(44 + count * 2)
  bytes.write('RIFF')
  bytes.writeUInt32LE(bytes.length - 8, 4)
  bytes.write('WAVEfmt ', 8)
  bytes.writeUInt32LE(16, 16)
  bytes.writeUInt16LE(1, 20)
  bytes.writeUInt16LE(1, 22)
  bytes.writeUInt32LE(rate, 24)
  bytes.writeUInt32LE(rate * 2, 28)
  bytes.writeUInt16LE(2, 32)
  bytes.writeUInt16LE(16, 34)
  bytes.write('data', 36)
  bytes.writeUInt32LE(count * 2, 40)
  for (let i = 0; i < count; i++)
    bytes.writeInt16LE(Math.round(Math.sin((i * 220 * 2 * Math.PI) / rate) * 1000), 44 + i * 2)
  return {
    audio_base64: bytes.toString('base64'),
    mime: 'audio/wav',
    duration_ms: duration,
    voice: 'en-US-Ava:DragonHDLatestNeural',
    language: 'en-US',
    sdk_version: 'fixture-only',
    visemes: [
      { id: 0, offset_ms: 0 },
      { id: 2, offset_ms: duration / 30 },
      { id: 21, offset_ms: duration / 3 },
      { id: 0, offset_ms: duration - 100 },
    ],
  }
}
async function fixture(page: Page, holdReply = false) {
  const received: any[] = []
  const recognized: any[] = []
  await page.route('**/api/**', () => {
    throw Error('Unexpected API request escaped the offline conversation fixture')
  })
  await page.route('**/api/v1/conversation/status', (r) =>
    r.fulfill({ json: { configured: true, recognition: true } }),
  )
  await page.route('**/api/v1/speech/status', (r) => r.fulfill({ json: { configured: true } }))
  await page.route('**/api/v1/speech/synthesize', () => {
    throw Error('Prepared speech must never call synthesis again')
  })
  let publish: (value: any) => void = () => {}
  let disconnect = () => {}
  await page.routeWebSocket('**/api/v1/conversation/ws', (ws) => {
    disconnect = () => ws.close({ code: 1011, reason: 'offline connection loss' })
    ws.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      received.push(event)
      const send = (type: string, text = '', data?: any, id = event.turn_id) =>
        ws.send(
          JSON.stringify({ type, conversation_id: event.conversation_id, turn_id: id, text, data }),
        )
      if (event.type === 'hello') {
        send('ready', '', [])
        publish = (value) =>
          ws.send(JSON.stringify({ conversation_id: event.conversation_id, ...value }))
      }
      if (event.type === 'submit') throw Error('Microphone-only UI must not submit typed text')
      if (event.type === 'mic_start') send('listening')
    })
  })
  return {
    received,
    disconnect: () => disconnect(),
    recognized,
    publish: (value: any) => publish(value),
    recognize(text: string) {
      const turn = { turn_id: `fixture-recognized-${recognized.length + 1}`, text }
      recognized.push(turn)
      publish({ type: 'transcript_final', ...turn })
      publish({ type: 'accepted', ...turn })
      if (!holdReply) {
        publish({ type: 'reply_delta', turn_id: turn.turn_id, text: 'Hello. ' })
        publish({ type: 'reply_delta', turn_id: turn.turn_id, text: 'How can I help?' })
        publish({
          type: 'reply_complete',
          turn_id: turn.turn_id,
          text: 'Hello. How can I help?',
          data: { spoken: 'Hello. How can I help?' },
        })
        publish({
          type: 'prepared_speech',
          turn_id: turn.turn_id,
          text: 'Hello. How can I help?',
          data: preparedAudio(),
        })
      }
      return turn
    },
  }
}

test('progressive speech starts before complete model text, plays once in order, and drains', async ({
  page,
}, info) => {
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit has no Web Audio')
  const f = await fixture(page, true)
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('heading', { name: 'Chat with Syntelle' }).click()
  const turn = f.recognize('A progressive offline reply.')
  await expect(page.locator('.microphone-control')).toHaveAttribute('data-state', 'preparing')
  await page.screenshot({ path: info.outputPath('preparing-reply.png') })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.locator('.microphone-progress')).toHaveCSS('animation-name', 'none')
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  const text = 'This opening phrase is ready. The rest follows.'
  f.publish({ type: 'reply_delta', turn_id: turn.turn_id, text: 'This opening phrase is ready. ' })
  await expect(page.getByText('This opening phrase is ready.', { exact: true })).toBeVisible()
  await expect(page.getByText(text, { exact: true })).toHaveCount(0)
  const began = Date.now()
  const part = (index: number) => ({
    type: 'speech_part',
    turn_id: turn.turn_id,
    data: {
      speech: preparedAudio(),
      text: index ? 'The rest follows.' : 'This opening phrase is ready.',
      index,
      final: index === 1,
      offset_ms: index * 3000,
    },
  })
  f.publish(part(0))
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible({ timeout: 1500 })
  expect(Date.now() - began).toBeLessThan(1500)
  await expect(page.locator('.microphone-control')).toHaveAttribute('data-state', 'speaking')
  await expect(page.locator('.microphone-control')).toHaveAttribute('data-capture', 'false')
  await page.screenshot({ path: info.outputPath('speaking-reply.png') })
  // Audio is already playing; neither the second phrase nor completion existed.
  expect(f.received.filter((e) => e.type === 'playback_started')).toHaveLength(1)
  f.publish({ type: 'reply_delta', turn_id: turn.turn_id, text: 'The rest follows.' })
  f.publish({ type: 'reply_complete', turn_id: turn.turn_id, text, data: { spoken: text } })
  f.publish(part(1))
  f.publish(part(1))
  f.publish({ type: 'speech_complete', turn_id: turn.turn_id, data: { parts: 2 } })
  await expect
    .poll(() => f.received.filter((e) => e.type === 'playback_started').length, { timeout: 5000 })
    .toBe(2)
  await expect
    .poll(() => f.received.filter((e) => e.type === 'playback_ended').length, { timeout: 5000 })
    .toBe(1)
  expect(f.received.filter((e) => e.type === 'playback_started')).toHaveLength(2)
  await test.info().attach('progressive-local-playback', {
    body: JSON.stringify({ first_part_to_speaking_ms_upper_bound: 1500, provider_calls: 0 }),
    contentType: 'application/json',
  })
})

test('long phrase queue drains completely and connection loss is explicit', async ({
  page,
}, info) => {
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit has no Web Audio')
  const f = await fixture(page, true)
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('heading', { name: 'Chat with Syntelle' }).click()
  const turn = f.recognize('A complete long offline reply.')
  const text = 'A long reply with eight native phrase recordings.'
  f.publish({ type: 'reply_complete', turn_id: turn.turn_id, text, data: { spoken: text } })
  for (let index = 0; index < 8; index++)
    f.publish({
      type: 'speech_part',
      turn_id: turn.turn_id,
      data: { speech: preparedAudio(600), text, index, final: index === 7, offset_ms: index * 600 },
    })
  f.publish({ type: 'speech_complete', turn_id: turn.turn_id, data: { parts: 8 } })
  await expect
    .poll(() => f.received.filter((e) => e.type === 'playback_started').length, { timeout: 10000 })
    .toBe(8)
  await expect.poll(() => f.received.filter((e) => e.type === 'playback_ended').length).toBe(1)
  const next = f.recognize('An interrupted reply must be visible.')
  f.publish({
    type: 'speech_part',
    turn_id: next.turn_id,
    data: { speech: preparedAudio(), text, index: 0, final: false, offset_ms: 0 },
  })
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  f.disconnect()
  await expect(
    page.getByText(
      'The reply was interrupted because the connection closed. Turn on the microphone to reconnect.',
      { exact: true },
    ),
  ).toBeVisible()
  await expect(page.getByText('Speaking', { exact: true })).toHaveCount(0)
  await page.screenshot({ path: info.outputPath('explicit-disconnection.png') })
})

test('missing audio part cannot be reported as successful completion', async ({ page }, info) => {
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit has no Web Audio')
  const f = await fixture(page, true)
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('heading', { name: 'Chat with Syntelle' }).click()
  const turn = f.recognize('An incomplete offline delivery.')
  f.publish({
    type: 'speech_part',
    turn_id: turn.turn_id,
    data: {
      speech: preparedAudio(),
      text: 'A partial reply.',
      index: 0,
      final: false,
      offset_ms: 0,
    },
  })
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  f.publish({ type: 'speech_complete', turn_id: turn.turn_id, data: { parts: 2 } })
  await expect(
    page.getByText('The reply ended before all audio arrived. Please ask to continue.', {
      exact: true,
    }),
  ).toBeVisible()
  expect(f.received.filter((e) => e.type === 'playback_ended')).toHaveLength(0)
})

test('interruption discards queued progressive phrases and late delivery', async ({
  page,
}, info) => {
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit has no Web Audio')
  const f = await fixture(page, true)
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('heading', { name: 'Chat with Syntelle' }).click()
  const turn = f.recognize('Interrupt the first phrase.')
  const part = (index: number) => ({
    type: 'speech_part',
    turn_id: turn.turn_id,
    data: {
      speech: preparedAudio(),
      text: 'An offline phrase.',
      index,
      final: index === 1,
      offset_ms: index * 3000,
    },
  })
  f.publish(part(0))
  f.publish(part(1))
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Stop reply' }).click()
  f.publish(part(2))
  f.publish({ type: 'speech_complete', turn_id: turn.turn_id })
  await expect(page.getByText('Speaking', { exact: true })).toHaveCount(0)
  // A delayed phrase must not start after the stopped first source's duration.
  await page.waitForTimeout(3100)
  expect(f.received.filter((e) => e.type === 'playback_started')).toHaveLength(1)
})

test('late interruption warnings preserve the next turn and higher-priority errors', async ({
  page,
}) => {
  const f = await fixture(page, true)
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  f.recognize('First offline turn')
  await expect.poll(() => f.recognized.length).toBe(1)
  const first = f.recognized[0]
  await page.getByRole('button', { name: 'Stop reply' }).click()
  const warning = 'Playback stopped, but this change could not be saved.'
  f.publish({ type: 'cancelled', turn_id: first.turn_id })
  f.publish({ type: 'persistence_warning', turn_id: first.turn_id, text: warning })
  await expect(page.getByText(warning, { exact: true })).toBeVisible()
  await expect(page.getByText('Reply interrupted', { exact: true })).toHaveCount(1)

  f.recognize('Second offline turn')
  await expect.poll(() => f.recognized.length).toBe(2)
  const second = f.recognized[1]
  f.publish({ type: 'transcript_final', turn_id: first.turn_id, text: first.text })
  f.publish({ type: 'persistence_warning', turn_id: first.turn_id, text: warning })
  await expect(page.getByText(warning, { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Stop reply' })).toBeVisible()
  await expect(page.getByText('Reply interrupted', { exact: true })).toHaveCount(1)
  f.publish({
    type: 'reply_delta',
    turn_id: second.turn_id,
    text: 'The next turn is still active.',
  })
  await expect(page.getByText('The next turn is still active.', { exact: true })).toBeVisible()

  f.publish({ type: 'error', turn_id: second.turn_id, text: 'Fixture provider stopped.' })
  await expect(page.getByText('Fixture provider stopped.', { exact: true })).toBeVisible()
  f.publish({ type: 'persistence_warning', turn_id: first.turn_id, text: warning })
  f.publish({ type: 'reply_delta', turn_id: first.turn_id, text: 'STALE WARNING TURN' })
  f.publish({
    type: 'prepared_speech',
    turn_id: first.turn_id,
    text: 'Obsolete',
    data: preparedAudio(),
  })
  f.publish({
    type: 'persistence_warning',
    conversation_id: 'another-session',
    turn_id: first.turn_id,
    text: 'UNRELATED WARNING',
  })
  await expect(page.getByText('Fixture provider stopped.', { exact: true })).toBeVisible()
  await expect(page.getByText(warning, { exact: true })).toHaveCount(0)
  await expect(page.getByText('STALE WARNING TURN')).toHaveCount(0)
  await expect(page.getByText('UNRELATED WARNING')).toHaveCount(0)
  await expect(page.getByText('Speaking', { exact: true })).toHaveCount(0)
})

test('long replies retain every paragraph without a shortened-audio notice', async ({
  page,
}, info) => {
  const f = await fixture(page, true)
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  f.recognize('Explain the subject in depth.')
  await expect.poll(() => f.recognized.length).toBe(1)
  const turn = f.recognized[0]
  const reply =
    Array.from(
      { length: 6 },
      (_, i) =>
        `Paragraph ${i + 1}. ${'A detailed explanation can continue for as long as the answer needs. '.repeat(5)}`,
    ).join('\n\n') + '\nThis is the complete final sentence.'
  const spoken = reply.split(/\s+/).join(' ')
  f.publish({ type: 'reply_delta', turn_id: turn.turn_id, text: reply })
  f.publish({ type: 'reply_complete', turn_id: turn.turn_id, text: reply, data: { spoken } })
  f.publish({ type: 'prepared_speech', turn_id: turn.turn_id, text: spoken, data: preparedAudio() })
  await expect(page.locator('.character-message .message-text')).toHaveText(reply)
  expect(await page.locator('.character-message .message-text').textContent()).toBe(reply)
  await expect(page.getByText('Audio uses a shortened version of this reply.')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeVisible()
  if (await page.getByRole('button', { name: 'Stop reply' }).isVisible())
    await page.getByRole('button', { name: 'Stop reply' }).click()
  await page.screenshot({ path: info.outputPath('complete-long-reply.png') })
})

test('Voice chat streams replies, interrupts stale audio and starts a new conversation', async ({
  page,
}, info) => {
  const f = await fixture(page)
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Chat with Syntelle' })).toBeVisible()
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  f.recognize('Hello from a recognized turn')
  await expect(page.getByText('Hello. How can I help?', { exact: true })).toBeVisible()
  if (!info.project.name.startsWith('webkit'))
    await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  const submitted = f.recognized[0]
  expect(submitted).toBeTruthy()
  if (await page.getByRole('button', { name: 'Stop reply' }).isVisible())
    await page.getByRole('button', { name: 'Stop reply' }).click()
  f.publish({ type: 'reply_delta', turn_id: submitted.turn_id, text: 'STALE CONTENT' })
  f.publish({
    type: 'prepared_speech',
    turn_id: submitted.turn_id,
    text: 'Obsolete',
    data: preparedAudio(),
  })
  await expect(page.getByText('STALE CONTENT')).toHaveCount(0)
  await page.getByRole('button', { name: 'New chat', exact: true }).click()
  f.recognize('A fresh Syntelle conversation.')
  await expect(page.getByText('Hello. How can I help?', { exact: true })).toBeVisible()
  if (await page.getByRole('button', { name: 'Stop reply' }).isVisible())
    await page.getByRole('button', { name: 'Stop reply' }).click()
  await expect
    .poll(() => f.received.filter((e) => e.type === 'hello').length)
    .toBeGreaterThanOrEqual(2)
  const hellos = f.received.filter((e) => e.type === 'hello')
  expect(hellos[0].conversation_id).not.toEqual(hellos.at(-1).conversation_id)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await page
    .getByLabel('How should this character respond?')
    .fill('Use short, helpful Spanish replies.')
  await page.getByRole('button', { name: 'Close customization' }).click()
  await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toHaveCount(0)
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(errors).toEqual([])
  await page.screenshot({ path: info.outputPath('voice-conversation.png'), fullPage: true })
})

test('microphone uses the worklet, never submits from energy, and stops when hidden', async ({
  page,
}, info) => {
  test.skip(
    info.project.name.startsWith('webkit'),
    'Windows WebKit lacks Web Audio; explicit fallback is checked separately',
  )
  const f = await fixture(page)
  await page.addInitScript(() => {
    const state = window as any
    state.captureTracks = []
    navigator.mediaDevices.getUserMedia = async () => {
      const audio = new AudioContext()
      const output = audio.createMediaStreamDestination()
      const oscillator = audio.createOscillator()
      oscillator.frequency.value = 220
      const gain = audio.createGain()
      gain.gain.value = 0
      state.microphoneGain = gain
      oscillator.connect(gain)
      gain.connect(output)
      oscillator.start()
      state.captureTracks.push(...output.stream.getTracks())
      return output.stream
    }
  })
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.getByRole('button', { name: 'Enable microphone' }).click()
  await expect(page.getByRole('button', { name: 'Microphone on' })).toBeVisible()
  await expect.poll(() => f.received.filter((e) => e.type === 'audio').length).toBeGreaterThan(2)
  const meter = page.locator('.microphone-control')
  const level = () =>
    meter.evaluate((element) =>
      Number((element as HTMLElement).style.getPropertyValue('--input-level')),
    )
  await expect(meter).toHaveAttribute('data-state', 'listening')
  await expect.poll(level).toBe(0)
  await page.screenshot({ path: info.outputPath('listening-silence.png') })
  await page.evaluate(() => {
    ;(window as any).microphoneGain.gain.value = 0.02
  })
  await expect.poll(level).toBeGreaterThan(0.3)
  await expect.poll(level).toBeLessThan(0.4)
  await page.evaluate(() => {
    ;(window as any).microphoneGain.gain.value = 0.2
  })
  await expect.poll(level).toBe(1)
  await page.screenshot({ path: info.outputPath('listening-volume.png') })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.locator('.microphone-halo')).toHaveCSS('transform', 'none')
  await expect(page.locator('.microphone-halo')).toHaveCSS('opacity', '0.6')
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await page.evaluate(() => {
    ;(window as any).microphoneGain.gain.value = 0
  })
  await expect.poll(level).toBe(0)
  const microphoneTurn = f.received.find((event) => event.type === 'mic_start')
  f.publish({
    type: 'transcript_partial',
    turn_id: microphoneTurn.turn_id,
    text: 'A recognized phrase',
  })
  await expect(page.locator('.partial-transcript')).toHaveText('A recognized phrase…')
  const packet = f.received.find((e) => e.type === 'audio')
  expect(Buffer.from(packet.audio_base64, 'base64').length).toBeGreaterThanOrEqual(3200)
  expect(f.received.filter((e) => e.type === 'submit')).toHaveLength(0)
  f.recognize('Reply while the microphone is enabled.')
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  const began = Date.now()
  await page.evaluate(() => {
    ;(window as any).microphoneGain.gain.value = 0.5
  })
  await expect.poll(() => f.received.some((event) => event.type === 'activity')).toBe(true)
  const interruptedAfter = Date.now() - began
  expect(interruptedAfter).toBeLessThan(500)
  // Energy alone is not a confirmed new utterance and must not cut the reply.
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  expect(
    f.received.filter((event) => event.type === 'cancel' && event.reason === 'microphone activity'),
  ).toHaveLength(0)
  f.recognize('A confirmed new utterance interrupts this reply.')
  await expect(
    page.getByText('A confirmed new utterance interrupts this reply.', { exact: true }),
  ).toBeVisible()
  await expect(page.getByText('Speaking', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Stop reply' }).click()
  await expect(page.getByRole('button', { name: 'Stop reply' })).toHaveCount(0)
  expect(f.received.filter((event) => event.type === 'submit')).toHaveLength(0)
  await page.screenshot({ path: info.outputPath('interrupted-microphone.png'), fullPage: true })
  await test.info().attach('local-interruption-delay', {
    body: JSON.stringify({ milliseconds: interruptedAfter, fixture: true }),
    contentType: 'application/json',
  })

  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect
    .poll(() =>
      page.evaluate(() =>
        (window as any).captureTracks.every((t: MediaStreamTrack) => t.readyState === 'ended'),
      ),
    )
    .toBe(true)
  await page.evaluate(() => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: false })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeVisible()
  await expect.poll(level).toBe(0)
  expect(f.received.filter((e) => e.type === 'mic_start')).toHaveLength(1)
  await page.getByRole('button', { name: 'Enable microphone' }).click()
  await expect(page.getByRole('button', { name: 'Microphone on' })).toBeVisible()
  await page.getByRole('button', { name: 'Microphone on' }).click()
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeVisible()
  expect(
    await page.evaluate(() =>
      (window as any).captureTracks.every(
        (track: MediaStreamTrack) => track.readyState === 'ended',
      ),
    ),
  ).toBe(true)
  await expect.poll(level).toBe(0)
  expect(f.received.filter((event) => event.type === 'mic_start')).toHaveLength(2)
  expect(f.received.some((event) => event.type === 'mic_stop')).toBe(true)
  await page.getByRole('button', { name: 'Enable microphone' }).click()
  await expect(page.getByRole('button', { name: 'Microphone on' })).toBeVisible()
  await page.evaluate(() => {
    ;(window as any).microphoneGain.gain.value = 0.5
  })
  await expect.poll(level).toBe(1)
  await page.evaluate(() => {
    ;(window as any).captureTracks.at(-1).dispatchEvent(new Event('ended'))
  })
  await expect(page.getByRole('alert')).toContainText('Microphone disconnected.')
  await expect.poll(level).toBe(0)
  await expect(meter).toHaveAttribute('data-state', 'error')
  await expect(page.locator('.microphone-progress')).toHaveCSS('animation-name', 'none')
})

test('new chat cancels a pending microphone permission request safely', async ({ page }, info) => {
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit lacks Web Audio')
  const f = await fixture(page)
  await page.addInitScript(() => {
    const state = window as any
    navigator.mediaDevices.getUserMedia = async () => {
      const audio = new AudioContext()
      const output = audio.createMediaStreamDestination()
      state.pendingTracks = output.stream.getTracks()
      await new Promise<void>((resolve) => {
        state.allowMicrophone = resolve
      })
      return output.stream
    }
  })
  await page.goto('/')
  const microphone = page.getByRole('button', { name: 'Enable microphone' })
  await expect(microphone).toBeEnabled()
  await microphone.click()
  await expect(microphone).toHaveAttribute('aria-busy', 'true')
  await expect(page.locator('.microphone-control')).toHaveAttribute('data-state', 'starting')
  await page.screenshot({ path: info.outputPath('permission-startup.png') })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.locator('.microphone-progress')).toHaveCSS('animation-name', 'none')
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await expect(microphone).toBeDisabled()
  await expect
    .poll(() => page.evaluate(() => typeof (window as any).allowMicrophone))
    .toBe('function')
  await page.getByRole('button', { name: 'New chat', exact: true }).click()
  await page.evaluate(() => {
    ;(window as any).allowMicrophone()
  })
  await expect(microphone).toBeEnabled()
  await expect(microphone).toHaveAttribute('aria-pressed', 'false')
  expect(
    await page.evaluate(() =>
      (window as any).pendingTracks.every(
        (track: MediaStreamTrack) => track.readyState === 'ended',
      ),
    ),
  ).toBe(true)
  expect(f.received.filter((event) => event.type === 'mic_start')).toHaveLength(0)
})

test('stateful resampling rejects aliases and activity requires 120 ms', () => {
  const wave = (frequency: number) =>
    Float32Array.from(
      { length: 48000 },
      (_, i) => Math.sin((i * 2 * Math.PI * frequency) / 48000) * 0.7,
    )
  const process = (input: Float32Array, chunk: number) => {
    const r = new PCMResampler(48000)
    const output: number[] = []
    for (let i = 0; i < input.length; i += chunk)
      output.push(...r.push(input.subarray(i, i + chunk)))
    return output
  }
  const low = process(wave(1000), 128),
    differentChunks = process(wave(1000), 997),
    high = process(wave(12000), 128)
  expect(low.length).toBeGreaterThan(15980)
  expect(low).toEqual(differentChunks)
  const rms = (x: number[]) =>
    Math.sqrt(x.slice(100).reduce((sum, v) => sum + (v / 32768) ** 2, 0) / (x.length - 100))
  expect(rms(low)).toBeGreaterThan(0.45)
  expect(rms(high)).toBeLessThan(0.015)
  const gate = new ActivityGate()
  expect(gate.update(new Int16Array(1600).fill(8000))).toBe(false)
  expect(gate.update(new Int16Array(320).fill(8000))).toBe(true)
  expect(gate.update(new Int16Array(320).fill(8000))).toBe(false)
})

test('microphone errors distinguish blocked access, missing devices and unavailable capture', async ({
  page,
}, info) => {
  test.skip(info.project.name.startsWith('webkit'), 'Windows WebKit lacks Web Audio')
  const f = await fixture(page)
  await page.addInitScript(() => {
    navigator.mediaDevices.getUserMedia = async () => {
      throw new DOMException('Fixture capture failure', (window as any).fixtureMicrophoneError)
    }
  })
  await page.goto('/')
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  for (const [name, message] of [
    ['NotAllowedError', 'Microphone access is blocked by the browser or system.'],
    ['NotFoundError', 'No microphone was found.'],
    ['NotReadableError', 'The microphone could not be opened.'],
  ]) {
    await page.evaluate((value) => {
      ;(window as any).fixtureMicrophoneError = value
    }, name)
    await page.getByRole('button', { name: 'Enable microphone' }).click()
    await expect(page.getByRole('alert')).toContainText(message!)
    await expect(page.locator('.microphone-control')).toHaveAttribute('data-state', 'error')
    await expect(page.locator('.microphone-progress')).toHaveCSS('animation-name', 'none')
    await expect(page.getByRole('button', { name: 'Enable microphone' })).toHaveAttribute(
      'aria-pressed',
      'false',
    )
  }
  expect(
    f.received.filter((event) => event.type === 'mic_start' || event.type === 'audio'),
  ).toHaveLength(0)
  await page.screenshot({ path: info.outputPath('microphone-capture-error.png'), fullPage: true })
})

test('voice controls stay visible across themes, layouts and long history', async ({
  page,
}, info) => {
  test.setTimeout(60_000)
  const f = await fixture(page)
  await page.goto('/')
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeEnabled()
  for (const [width, height] of [
    [1280, 720],
    [1440, 900],
    [390, 844],
  ]) {
    await page.setViewportSize({ width: width!, height: height! })
    for (const theme of ['light', 'dark']) {
      await page.emulateMedia({ colorScheme: theme as 'light' | 'dark' })
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme)
      await expect(page.getByRole('button', { name: 'Send', exact: true })).toHaveCount(0)
      await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeInViewport({
        ratio: 1,
      })
      await expect(page.getByLabel('Language', { exact: true })).toHaveCount(0)
      await expect(page.getByRole('button', { name: 'Café', exact: true })).toHaveCount(0)
      await expect(page.getByRole('button', { name: 'Woodland', exact: true })).toHaveCount(0)
      await expect(page.getByRole('heading', { name: 'Start a conversation' })).toBeInViewport({
        ratio: 1,
      })
      await expect(page.getByRole('group', { name: 'Conversation starters' })).toHaveCount(0)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true,
      )
      if (width! >= 900) {
        const stage = await page.locator('.stage-panel').boundingBox()
        const chat = await page.getByRole('region', { name: 'Chat workspace' }).boundingBox()
        expect(Math.abs(stage!.height - chat!.height)).toBeLessThan(1)
        expect(stage!.width / (stage!.width + chat!.width)).toBeCloseTo(0.55, 2)
      } else {
        await expect(page.getByRole('img', { name: 'Animated character', exact: true })).toHaveCSS(
          'object-fit',
          'contain',
        )
      }
      await page.screenshot({
        path: info.outputPath(`refresh-${width}-${height}-${theme}.png`),
        fullPage: true,
      })
    }
  }
  const turns = Array.from({ length: 10 }, (_, i) => ({
    id: `fixture-history-${i}`,
    sequence: i,
    status: 'ready',
    user: `Question ${i}: ${'A long message with several lines. '.repeat(8)}`,
    reply: `Response ${i}: ${'A longer reply that stays inside the transcript. '.repeat(12)}`,
    spoken: '',
  }))
  f.publish({ type: 'ready', data: turns })
  await expect(page.getByRole('log').locator('.exchange')).toHaveCount(10)
  const log = page.getByRole('log')
  await log.evaluate((element) => {
    element.scrollTop = 0
  })
  f.publish({
    type: 'ready',
    data: [...turns, { ...turns[0], id: 'fixture-appended', sequence: 11 }],
  })
  await expect(log.locator('.exchange')).toHaveCount(11)
  expect(await log.evaluate((element) => element.scrollTop)).toBe(0)
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeInViewport({ ratio: 1 })
  expect(f.received.filter((event) => event.type === 'submit')).toHaveLength(0)
  await page.screenshot({ path: info.outputPath('refresh-history.png'), fullPage: true })
})

test('voice-only workspace has no writing controls and preserves long recognized text', async ({
  page,
}) => {
  const f = await fixture(page, true)
  await page.goto('/')
  const microphone = page.getByRole('button', { name: 'Enable microphone' })
  await expect(microphone).toBeEnabled()
  await expect(page.getByRole('textbox')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toHaveCount(0)
  await expect(page.getByRole('group', { name: 'Conversation starters' })).toHaveCount(0)
  await expect(page.locator('#message-count')).toHaveCount(0)
  await microphone.focus()
  await expect(microphone).toBeFocused()
  const text = 'Explain this in detail: '.repeat(80) + 'Gracias. 日本語 también.'
  f.recognize(text)
  await expect(page.locator('.user-message .message-text')).toHaveText(text)
  await expect(page.getByRole('button', { name: 'Stop reply' })).toBeVisible()
  await page.getByRole('button', { name: 'Stop reply' }).click()
  await expect(page.getByRole('button', { name: 'Stop reply' })).toHaveCount(0)
  await expect(microphone).toBeFocused()
  expect(f.received.filter((event) => event.type === 'submit')).toHaveLength(0)
})

test('mobile customization restores focus and voice controls after keyboard resize', async ({
  page,
}, info) => {
  test.setTimeout(40_000)
  await page.setViewportSize({ width: 390, height: 844 })
  await fixture(page)
  await page.addInitScript(() => {
    const viewport = new EventTarget()
    const state = window as any
    state.fixtureViewportHeight = 844
    Object.defineProperties(viewport, {
      height: { get: () => state.fixtureViewportHeight },
      offsetTop: { get: () => 0 },
    })
    Object.defineProperty(window, 'visualViewport', { value: viewport })
  })
  await page.goto('/')
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeEnabled()
  const customize = page.getByRole('button', { name: 'Customize', exact: true })
  await customize.click()
  await page.getByLabel('How should this character respond?').focus()
  await page.evaluate(() => {
    ;(window as any).fixtureViewportHeight = 440
    window.visualViewport!.dispatchEvent(new Event('resize'))
  })
  await expect(page.locator('.demo-workspace')).toHaveClass(/keyboard-open/)
  await expect(page.locator('.stage-panel')).not.toBeVisible()
  await expect(page.getByRole('dialog', { name: 'Customize', exact: true })).toBeVisible()
  await page.screenshot({
    path: info.outputPath('refresh-keyboard-simulation.png'),
    fullPage: true,
  })
  await page.evaluate(() => {
    ;(window as any).fixtureViewportHeight = 844
    window.visualViewport!.dispatchEvent(new Event('resize'))
  })
  await expect(page.locator('.stage-panel')).toBeVisible()
  await page.getByRole('button', { name: 'Close customization' }).press('Escape')
  await expect(customize).toBeFocused()
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeInViewport({ ratio: 1 })
  await expect(page.getByRole('region', { name: 'Syntelle character preview' })).toHaveAttribute(
    'aria-busy',
    'false',
  )
  await page.screenshot({ path: info.outputPath('refresh-mobile-Syntelle.png'), fullPage: true })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await customize.click()
  await expect(page.getByLabel('Natural movement', { exact: true })).toBeDisabled()
})

test('unavailable voice setup disables capture and keeps customization reachable', async ({
  page,
}) => {
  await fixture(page)
  await page.route('**/api/v1/conversation/status', (route) =>
    route.fulfill({ json: { configured: false, recognition: false } }),
  )
  await page.goto('/')
  await expect(page.getByText('Chat setup needed', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeDisabled()
  await expect(page.getByRole('textbox')).toHaveCount(0)
  await page.getByRole('button', { name: 'Customize', exact: true }).click()
  await expect(page.getByRole('dialog', { name: 'Customize', exact: true })).toBeVisible()
})

test('missing recognition has an explicit voice setup status', async ({ page }) => {
  await fixture(page)
  await page.route('**/api/v1/conversation/status', (route) =>
    route.fulfill({ json: { configured: true, recognition: false } }),
  )
  await page.goto('/')
  await expect(page.getByText('Voice setup needed', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeDisabled()
})

test('unsupported browser reports voice failure without offering typed chat', async ({ page }) => {
  await fixture(page)
  await page.addInitScript(() => {
    ;(window as any).AudioContext = undefined
  })
  await page.goto('/')
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toBeEnabled()
  await page.getByRole('button', { name: 'Enable microphone' }).click()
  await expect(page.getByRole('alert')).toContainText('not supported in this browser')
  await expect(page.getByRole('textbox')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Enable microphone' })).toHaveAttribute(
    'aria-pressed',
    'false',
  )
})

test('visual RMS meter maps the specified decibel window independently of activity', () => {
  const packet = (db: number) => Array(1600).fill(32768 * 10 ** (db / 20))
  expect(inputLevel([])).toBe(0)
  expect(inputLevel(Array(1600).fill(0))).toBe(0)
  expect(inputLevel(packet(-60))).toBe(0)
  expect(inputLevel(packet(-48))).toBeCloseTo(0, 8)
  expect(inputLevel(packet(-33))).toBeCloseTo(0.5, 8)
  expect(inputLevel(packet(-18))).toBeCloseTo(1, 8)
  expect(inputLevel(packet(0))).toBe(1)
})
