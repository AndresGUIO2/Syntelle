import { expect, test, type Page } from '@playwright/test'

const characterId = 'char_' + 'a'.repeat(32)
const baseAppearance = {
  placement: 'floating',
  position: 'bottom-right',
  theme: 'system',
  language: 'en-US',
  launcher_label: 'Start conversation',
}
async function fixture(
  page: Page,
  options: {
    empty?: boolean
    copies?: number
    uncertain?: boolean
    role?: string
    workspaceFailure?: 'rejected' | 'uncertain'
  } = {},
) {
  const writes: { path: string; body: Record<string, unknown> }[] = []
  const workspace = {
    id: 'workspace-one',
    name: 'Studio',
    role: options.role ?? 'owner',
    suspended: false,
  }
  const workspaces = options.empty ? [] : [workspace]
  const publications: Record<string, unknown>[] = Array.from(
    { length: options.copies ?? 0 },
    (_, i) => ({
      id: 'publication-' + i,
      workspace_id: workspace.id,
      character_id: characterId,
      name: 'Guide copy ' + i,
      package_hash: 'same-source',
      settings: {},
      created_at: '2026-11-01T00:00:00Z',
    }),
  )
  const integrations: Record<string, unknown>[] = []
  let failed = false
  await page.route('https://**', (route) => route.abort())
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname,
      method = route.request().method()
    const body = method === 'GET' ? {} : (route.request().postDataJSON() ?? {})
    if (method !== 'GET') writes.push({ path, body })
    const reply = (data: unknown, status = 200) =>
      route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(data) })
    if (path === '/api/v1/auth/me')
      return reply({ account: { id: 'owner-one', name: 'Alex', email: 'alex@example.test' } })
    if (path === '/api/v1/auth/status')
      return reply({
        email: { configured: true },
        google: { configured: false },
        github: { configured: false },
      })
    if (path === '/api/v1/admin/eligibility') return reply({}, 403)
    if (path === '/api/v1/workspaces' && method === 'GET') return reply({ items: workspaces })
    if (path === '/api/v1/workspaces' && method === 'POST') {
      if (options.workspaceFailure === 'rejected' && !failed) {
        failed = true
        return reply({ error: 'Choose another workspace name.' }, 400)
      }
      workspace.name = body.name
      workspaces.push(workspace)
      if (options.workspaceFailure === 'uncertain' && !failed) {
        failed = true
        return route.abort('connectionreset')
      }
      return reply(workspace, 201)
    }
    if (path === '/api/v1/characters')
      return reply({
        items: [
          {
            id: characterId,
            name: 'Personal guide',
            status: 'ready',
            portrait_available: false,
            created_at: '2026-11-01T00:00:00Z',
          },
        ],
      })
    if (path.endsWith('/publications') && method === 'GET') return reply({ items: publications })
    if (path.endsWith('/publications') && method === 'POST') {
      const item = {
        id: 'publication-new',
        workspace_id: workspace.id,
        character_id: body.creation_id,
        name: body.name,
        settings: body.settings,
        package_hash: 'same-source',
        created_at: '2026-11-01T00:00:00Z',
      }
      publications.push(item)
      return reply(item, 201)
    }
    if (path.endsWith('/integrations') && method === 'GET') return reply({ items: integrations })
    if (path.endsWith('/integrations') && method === 'POST') {
      const item = {
        ...body,
        id: 'integration-new',
        workspace_id: workspace.id,
        widget_id: 'widget-fixture',
        appearance: body.appearance ?? baseAppearance,
        revision: 1,
        published_revision: 0,
        enabled: false,
      }
      integrations.push(item)
      if (options.uncertain && !failed) {
        failed = true
        return route.abort('connectionreset')
      }
      return reply(item, 201)
    }
    if (path.endsWith('/publish')) {
      Object.assign(integrations[0]!, { published_revision: 1, enabled: true })
      return reply(integrations[0])
    }
    if (path.endsWith('/keys')) return reply({ items: [] })
    return reply({ items: [] })
  })
  return { writes, publications, integrations }
}
async function start(page: Page) {
  await page.getByRole('button', { name: 'Connect a website', exact: true }).click()
}
async function website(page: Page) {
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
  await page.getByRole('textbox', { name: 'Connection name', exact: true }).fill('Studio website')
  await page
    .getByRole('textbox', { name: 'Website address', exact: true })
    .fill('https://studio.example')
  await page.getByRole('button', { name: 'Review connection', exact: true }).click()
}
async function navigate(page: Page, name: string) {
  const open = page.getByRole('button', { name: 'Open navigation', exact: true })
  if (await open.isVisible()) await open.click()
  await page.getByRole('link', { name, exact: true }).click()
}

test('first connection creates workspace and approved copy, saves draft, then publishes separately', async ({
  page,
}, info) => {
  const state = await fixture(page, { empty: true })
  await page.goto('/console/integrations')
  await expect(page.getByRole('textbox', { name: 'Workspace name' })).toHaveCount(0)
  expect(state.writes).toHaveLength(0)
  await start(page)
  await page.getByRole('textbox', { name: 'Workspace name' }).fill('Design Studio')
  await page.getByRole('button', { name: 'Create workspace and continue' }).click()
  await page.getByRole('combobox', { name: 'Character', exact: true }).selectOption(characterId)
  await website(page)
  const save = page.getByRole('button', { name: 'Save draft', exact: true })
  await expect(save).toBeDisabled()
  await page.getByRole('checkbox').check()
  await page.screenshot({ path: info.outputPath('integration-review.png'), fullPage: true })
  await save.click()
  await expect(page.getByRole('button', { name: 'Publish', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Install the widget', exact: true })).toHaveCount(
    0,
  )
  expect(state.writes.map((item) => item.path.split('/').at(-1))).toEqual([
    'workspaces',
    'publications',
    'integrations',
  ])
  expect(state.writes[1]?.body.creation_id).toBe(characterId)
  await page.getByRole('button', { name: 'Publish', exact: true }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Install the widget', exact: true })).toBeVisible()
  await expect(
    page.getByText('This installation address works only on this computer.', { exact: false }),
  ).toBeVisible()
  await page.screenshot({ path: info.outputPath('integration-published.png'), fullPage: true })
  expect(state.writes.at(-1)?.path).toContain('/publish')
})

test('selected character intent validates ready identity and requires a choice between shared copies', async ({
  page,
}) => {
  const state = await fixture(page, { copies: 2 })
  await page.addInitScript(
    ({ characterId }) =>
      sessionStorage.setItem(
        'syntelle.console.connection-intent',
        JSON.stringify({
          accountId: 'owner-one',
          destination: '/console/integrations',
          characterId,
        }),
      ),
    { characterId },
  )
  await page.goto('/console/integrations')
  await expect(page.getByRole('combobox', { name: 'Character', exact: true })).toHaveValue(
    characterId,
  )
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('Choose which workspace copy')
  await page
    .getByRole('combobox', { name: 'Workspace copy', exact: true })
    .selectOption('publication-1')
  await website(page)
  await expect(page.getByRole('checkbox')).toHaveCount(0)
  await page.getByRole('button', { name: 'Save draft', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Publish', exact: true })).toBeVisible()
  expect(state.writes).toHaveLength(1)
  expect(state.writes[0]?.body.publication_id).toBe('publication-1')
})

test('unsent connection fields survive internal navigation without starting work', async ({
  page,
}) => {
  const state = await fixture(page)
  await page.goto('/console/integrations')
  await start(page)
  await page.getByRole('button', { name: 'Continue', exact: true }).click()
  await page.getByRole('textbox', { name: 'Connection name', exact: true }).fill('Unsent website')
  await page
    .getByRole('textbox', { name: 'Website address', exact: true })
    .fill('https://draft.example')
  await navigate(page, 'Characters')
  await navigate(page, 'Integrations')
  await expect(page.getByRole('textbox', { name: 'Connection name', exact: true })).toHaveValue(
    'Unsent website',
  )
  await expect(page.getByRole('textbox', { name: 'Website address', exact: true })).toHaveValue(
    'https://draft.example',
  )
  expect(state.writes).toHaveLength(0)
})

test('uncertain creation discovers saved connection without repeating copy or creation', async ({
  page,
}) => {
  const state = await fixture(page, { uncertain: true })
  await page.goto('/console/integrations')
  await start(page)
  await website(page)
  await page.getByRole('checkbox').check()
  await page.getByRole('button', { name: 'Save draft', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Check saved work', exact: true }).click()
  await page.getByRole('button', { name: 'Open saved connection: Studio website' }).click()
  await expect(page.getByRole('button', { name: 'Publish', exact: true })).toBeVisible()
  expect(state.writes.map((item) => item.path.split('/').at(-1))).toEqual([
    'publications',
    'integrations',
  ])
})

test('editor cannot create a website connection', async ({ page }) => {
  await fixture(page, { role: 'editor' })
  await page.goto('/console/integrations')
  await expect(page.getByRole('button', { name: 'Connect a website', exact: true })).toHaveCount(0)
  await expect(
    page.getByText('Ask a workspace owner or administrator to connect a website.'),
  ).toBeVisible()
})

test('definite workspace rejection allows correction without treating it as uncertain', async ({
  page,
}) => {
  const state = await fixture(page, { empty: true, workspaceFailure: 'rejected' })
  await page.goto('/console/integrations')
  await start(page)
  const name = page.getByRole('textbox', { name: 'Workspace name', exact: true })
  await name.fill('Rejected name')
  await page.getByRole('button', { name: 'Create workspace and continue' }).click()
  await expect(name).toBeEnabled()
  await expect(page.getByRole('button', { name: 'Check workspaces', exact: true })).toHaveCount(0)
  await name.fill('Corrected name')
  await page.getByRole('button', { name: 'Create workspace and continue' }).click()
  await expect(page.getByRole('combobox', { name: 'Character', exact: true })).toBeVisible()
  expect(state.writes.map((item) => item.body.name)).toEqual(['Rejected name', 'Corrected name'])
})

test('uncertain workspace creation requires explicit recovered workspace choice', async ({
  page,
}) => {
  const state = await fixture(page, { empty: true, workspaceFailure: 'uncertain' })
  await page.goto('/console/integrations')
  await start(page)
  await page
    .getByRole('textbox', { name: 'Workspace name', exact: true })
    .fill('Recovered workspace')
  await page.getByRole('button', { name: 'Create workspace and continue' }).click()
  await expect(page.getByRole('button', { name: 'Create workspace and continue' })).toBeDisabled()
  await expect(page.getByRole('combobox', { name: 'Character', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Check workspaces', exact: true }).click()
  await page
    .getByRole('button', { name: 'Use selected workspace: Recovered workspace', exact: true })
    .click()
  await expect(page.getByRole('combobox', { name: 'Character', exact: true })).toBeVisible()
  expect(state.writes).toHaveLength(1)
})

test('revoked integration permission reports an actionable handoff failure', async ({ page }) => {
  const state = await fixture(page, { role: 'editor' })
  await page.addInitScript(() =>
    sessionStorage.setItem(
      'syntelle.console.connection-intent',
      JSON.stringify({
        accountId: 'owner-one',
        destination: '/console/integrations',
        characterId: 'syntelle',
      }),
    ),
  )
  await page.goto('/console/integrations')
  await expect(page.getByRole('alert')).toContainText(
    'Your workspace role does not allow creating website connections.',
  )
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toHaveCount(0)
  expect(state.writes).toHaveLength(0)
})

test('Spanish connection journey uses the same explicit draft and sharing boundaries', async ({
  page,
}, info) => {
  const state = await fixture(page)
  await page.addInitScript(() => localStorage.setItem('syntelle.locale', 'es'))
  await page.goto('/console/integrations')
  await page.getByRole('button', { name: 'Conectar una web', exact: true }).click()
  await page.getByRole('button', { name: 'Continuar', exact: true }).click()
  await page
    .getByRole('textbox', { name: 'Nombre de la conexión', exact: true })
    .fill('Sitio del estudio')
  await page
    .getByRole('textbox', { name: 'Dirección de la web', exact: true })
    .fill('https://estudio.example')
  await page.getByRole('button', { name: 'Revisar conexión', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Guardar borrador', exact: true })).toBeDisabled()
  await page.locator('main').evaluate((element) => {
    element.scrollTop = 0
  })
  await page.screenshot({ path: info.outputPath('integration-review-spanish.png'), fullPage: true })
  expect(state.writes).toHaveLength(0)
})
