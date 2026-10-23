# Complete conversation replies

The owner rejected the AI's 500-character limit and the confusing shortened-audio message on October 20, 2026. This follow-up supersedes the demo's earlier response-length restriction; historical evidence and paid allowances remain unchanged.

The application prompt now asks for depth appropriate to the question, without a 500-character or one-to-three-sentence requirement. Both the default conversation instructions and the structured performance prompt use the complete answer. The model's output allocation increases to 4,096 tokens and the decoded stream safety bound increases to 64 KiB. These are finite transport/resource safeguards, not a request to shorten every answer. Incomplete provider responses remain explicit failures and are not silently presented as complete speech.

`SpokenText` only normalizes whitespace. It no longer cuts at 500 UTF-16 units, a sentence or a word. The transcript keeps the original paragraphs. The old notice compared the two strings directly, so it could claim audio was shortened merely because newlines or repeated spaces differed. That notice is removed.

## Preparing longer speech

Conversation speech is prepared in sequential parts of at most 3,000 UTF-16 units, preferring sentence/word boundaries and preserving every input byte across the complete sequence. This is an internal preparation size, not a limit on the reply. Each part uses the current voice, language and region, with its own committed request and immutable cached native audio/events. No retry, fallback voice or extra LLM request is introduced.

The server concatenates matching 24 kHz mono PCM16 samples into one derived WAV and offsets the native visemes onto the combined audio clock. Native events beyond a part's PCM endpoint are clamped only in the derived timeline; the original native event records remain unchanged. The derived `dragon-sequence` record stores part request keys, provider result IDs, WAV hashes and offsets, plus the assembled bytes. Replaying cached parts makes no additional provider call. Single-part results keep their existing response/cache behavior.

The existing frontend receives one prepared-speech payload and retains one pause/resume/stop clock. Contextual performance cues resolve against the complete decoded audio. Cancellation stops preparing further parts, and a provider failure does not trigger a retry or partial replacement. The overall preparation deadline is five minutes, with a one-minute provider deadline per conversation part. Sentence-boundary synthesis can introduce natural pauses/prosody resets between parts; no seamless phonetic continuation is claimed.

Azure documents finite [real-time synthesis limits](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/speech-services-quotas-and-limits); preparation parts keep long replies independent of the demo's former 500-character audio cap. Manual Read aloud retains its preceding policy. Existing historical trial budgets and the exhausted performance experiment are not changed or reused.

## Chat input follow-up

The owner also rejected the unexplained `500 left` counter. Chat now removes that counter, the textarea's 500-character restriction and both client/server submission guards. Long user messages reach the model intact, including Unicode and paragraphs. Empty messages remain blocked, the composer still grows only to 132 px before scrolling, and the existing 64 KiB WebSocket frame safeguard remains. This does not change manual Read aloud's separate input policy.

Focused Chromium/WebKit fixtures check starter focus without automatic sending, submission of the complete long draft, absent counter and mobile keyboard reachability. An isolated PostgreSQL Go fixture verifies acceptance and the exact long text received by the model. This follow-up uses no paid requests; source and review evidence are retained in `.tools/chat-input-36/`.

## Bounded verification

Local fixtures check complete text/Unicode reconstruction, a response exceeding 500 characters, multi-part PCM preservation and viseme offsets, cancellation, no retry after errors, cache reuse and immutable PostgreSQL sequence evidence. Browser fixtures check full multi-paragraph display, absence of the misleading notice even when speech whitespace differs, and existing interruption/Read aloud behavior. No paid model, speech or image request belongs to this change.

Evidence and source copies are in `.tools/full-replies-35/`, archived/restored with the generic PostgreSQL preparation archive. Prepared character assets, rejected facial laughter and its visual-acceptance status remain unchanged.
