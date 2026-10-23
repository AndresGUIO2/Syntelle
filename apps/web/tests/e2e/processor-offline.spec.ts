import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'node:url'

test('processor failure passes through the real API to the browser', async ({ page }) => {
  await page.goto('/?view=validation')
  const responsePromise = page.waitForResponse((response) =>
    response.url().endsWith('/api/v1/images/validate'),
  )
  await page
    .getByLabel('Portrait image', { exact: true })
    .setInputFiles(fileURLToPath(new URL('../fixtures/image.png', import.meta.url)))
  const response = await responsePromise
  expect(response.status()).toBe(503)
  expect((await response.json()).error.code).toBe('processor_unavailable')
  await expect(page.getByRole('alert')).toContainText('temporarily unavailable')
  await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible()
  await expect(page.getByRole('img', { name: /^Preview of / })).toHaveCount(0)
  const retryResponse = page.waitForResponse((response) =>
    response.url().endsWith('/api/v1/images/validate'),
  )
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  expect((await retryResponse).status()).toBe(503)
  await expect(page.getByRole('alert')).toContainText('temporarily unavailable')
})
