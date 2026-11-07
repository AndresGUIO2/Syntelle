import { reactive } from 'vue'
import type { WidgetAppearance } from './types'
import type { Workspace } from '../workspaces/useWorkspaces'

export interface WorkspaceCreationResult {
  workspace: Workspace | null
  uncertain: boolean
  error: string
}

export type IntegrationStep = 'workspace' | 'character' | 'website' | 'review'
export interface IntegrationJourney {
  accountId: string
  workspaceId: string
  open: boolean
  step: IntegrationStep
  workspaceName: string
  name: string
  characterId: string
  publicationId: string
  origins: string
  minutes: number
  appearance: WidgetAppearance
  sharingApproved: boolean
  busy: boolean
  creatingWorkspace: boolean
  error: string
  uncertain: '' | 'workspace' | 'publication' | 'integration'
}

// Only unsent presentation fields and identifiers live here. No browser storage or credentials.
const drafts = new Map<string, IntegrationJourney>()
const key = (owner: string, workspace: string) => `${owner}:${workspace}`
export function getIntegrationJourney(accountId: string, workspaceId = '') {
  const scope = key(accountId, workspaceId)
  let draft = drafts.get(scope)
  if (!draft) {
    draft = reactive<IntegrationJourney>({
      accountId,
      workspaceId,
      open: false,
      step: workspaceId ? 'character' : 'workspace',
      workspaceName: '',
      name: '',
      characterId: 'syntelle',
      publicationId: '',
      origins: '',
      minutes: 60,
      appearance: {
        placement: 'floating',
        position: 'bottom-right',
        theme: 'system',
        language: 'en-US',
        launcher_label: 'Start conversation',
      },
      sharingApproved: false,
      busy: false,
      creatingWorkspace: false,
      error: '',
      uncertain: '',
    })
    drafts.set(scope, draft)
  }
  return draft
}
export function bindIntegrationWorkspace(draft: IntegrationJourney, workspaceId: string) {
  // A response from a signed-out account must not recreate a cleared draft.
  if (drafts.get(key(draft.accountId, draft.workspaceId)) !== draft) return false
  drafts.delete(key(draft.accountId, draft.workspaceId))
  draft.workspaceId = workspaceId
  drafts.set(key(draft.accountId, workspaceId), draft)
  return true
}
export function discardIntegrationJourney(draft: IntegrationJourney) {
  if (drafts.get(key(draft.accountId, draft.workspaceId)) === draft)
    drafts.delete(key(draft.accountId, draft.workspaceId))
}
interface IntegrationEditDraft {
  name: string
  publication: string
  origins: string
  minutes: number
  appearance: WidgetAppearance
}
const edits = new Map<string, IntegrationEditDraft>()
export function getIntegrationEditDraft(key: string, initial: IntegrationEditDraft) {
  let existing = edits.get(key)
  if (!existing) {
    existing = reactive(initial)
    edits.set(key, existing)
  }
  return existing
}
export function clearIntegrationEditDraft(key: string) {
  edits.delete(key)
}
export function resetIntegrationJourney() {
  drafts.clear()
  edits.clear()
}

export function normalizeWebsiteOrigins(raw: string): string[] {
  const values = raw
    .split(/\r?\n/)
    .map((value) => value.trim())
    .filter(Boolean)
  if (!values.length || values.length > 20)
    throw new Error('Enter between one and twenty website addresses.')
  return [
    ...new Set(
      values.map((value) => {
        let parsed: URL
        try {
          parsed = new URL(value)
        } catch {
          throw new Error('Enter a complete website address, such as https://example.com.')
        }
        const local = ['localhost', '127.0.0.1', '[::1]'].includes(parsed.hostname)
        if (
          parsed.username ||
          parsed.password ||
          parsed.search ||
          parsed.hash ||
          parsed.hostname.includes('*') ||
          (parsed.pathname !== '/' && parsed.pathname !== '') ||
          (parsed.protocol !== 'https:' && !(local && parsed.protocol === 'http:'))
        )
          throw new Error(
            'Use an exact HTTPS website address without paths or wildcards. Localhost may use HTTP.',
          )
        return parsed.origin
      }),
    ),
  ]
}
