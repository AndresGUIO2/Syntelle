import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'

const fixture = (name: string) => readFileSync(new URL(`../fixtures/${name}`, import.meta.url))

for (const [extension, format, mime] of [
  ['jpg', 'JPEG', 'image/jpeg'],
  ['png', 'PNG', 'image/png'],
  ['webp', 'WEBP', 'image/webp'],
]) {
  test(`Go and FastAPI identify ${format} despite misleading extension and MIME`, async ({
    request,
  }) => {
    const buffer = fixture(`image.${extension}`)
    const response = await request.post('/api/v1/images/validate', {
      multipart: { image: { name: 'photo.txt', mimeType: 'text/plain', buffer } },
    })
    expect(response.status()).toBe(200)
    expect(await response.json()).toEqual({
      format,
      mime_type: mime,
      width: 32,
      height: 48,
      size_bytes: buffer.length,
    })
  })
}

test('exactly 5,000,000 bytes is accepted and one additional byte is rejected', async ({
  request,
}) => {
  const buffer = Buffer.alloc(5_000_000)
  fixture('image.png').copy(buffer)
  const accepted = await request.post('/api/v1/images/validate', {
    multipart: { image: { name: 'image.png', mimeType: 'image/png', buffer } },
  })
  expect(accepted.status()).toBe(200)
  expect((await accepted.json()).size_bytes).toBe(5_000_000)
  const rejected = await request.post('/api/v1/images/validate', {
    multipart: {
      image: {
        name: 'image.png',
        mimeType: 'image/png',
        buffer: Buffer.concat([buffer, Buffer.from([0])]),
      },
    },
  })
  expect(rejected.status()).toBe(413)
  expect((await rejected.json()).error.code).toBe('file_too_large')
})

test('a GIF named PNG is rejected by actual content', async ({ request }) => {
  const response = await request.post('/api/v1/images/validate', {
    multipart: {
      image: { name: 'image.png', mimeType: 'image/png', buffer: fixture('image.gif') },
    },
  })
  expect(response.status()).toBe(415)
  expect((await response.json()).error.code).toBe('unsupported_format')
})

test('corrupted PNG is rejected', async ({ request }) => {
  const response = await request.post('/api/v1/images/validate', {
    multipart: {
      image: { name: 'image.png', mimeType: 'image/png', buffer: fixture('corrupt.png') },
    },
  })
  expect(response.status()).toBe(422)
  expect((await response.json()).error.code).toBe('corrupt_image')
})

test('orientation-corrected dimensions survive the Go forwarding path', async ({ request }) => {
  const response = await request.post('/api/v1/images/validate', {
    multipart: {
      image: { name: 'rotated.jpg', mimeType: 'image/jpeg', buffer: fixture('rotated.jpg') },
    },
  })
  expect(response.status()).toBe(200)
  expect(await response.json()).toMatchObject({ width: 48, height: 32, format: 'JPEG' })
})

test('missing image is rejected and health is available', async ({ request }) => {
  expect((await request.get('/healthz')).status()).toBe(200)
  expect(
    (
      await request.post('/api/v1/images/validate', { multipart: { other: 'not a file' } })
    ).status(),
  ).toBe(400)
})
