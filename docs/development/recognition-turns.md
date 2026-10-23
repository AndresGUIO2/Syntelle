# Grouping microphone recognition into conversation turns

The owner reported that one spoken question was being sent as several messages: “What did you do?” followed by “In your daily routine.” The first request had already started and was interrupted by the second fragment. The cause was the immediate conversion of each Azure final recognition segment into a conversation turn.

## Current behavior

The SDK's existing 700 ms segmentation setting remains unchanged. The socket event loop now accumulates confirmed fragments and waits an additional 1,100 ms after the latest final result before creating a turn. A new speech-start or nonempty interim result suspends that deadline until the continuation resolves. A no-match result clears the unconfirmed guess and resumes waiting with only previously confirmed text. Interim guesses never become model input.

The combined text stays visible through the existing partial-transcript event. After settling, the server emits one final transcript and starts one model/speech request using the first confirmed result ID. Distinct SDK result IDs retain their text, including intentional repetition; repeat delivery of the same ID remains deduplicated. A later utterance after the settle boundary creates a separate turn. This is a pause-based turn boundary, not semantic certainty about whether the user has finished a thought. Provider/network timing and physical microphone behavior can affect perceived delay.

One reusable timer and the text buffer belong exclusively to the socket loop. No new worker, dependency, endpoint or migration is added. Microphone stop, recognition failure and disconnect discard unsent fragments and disable the timer. A legacy typed submission supersedes the pending microphone buffer. Existing playback-overlap/echo checks run before adding a fragment; activity still interrupts playback without creating a model turn. The browser ignores a replayed final transcript for an already-known turn, protecting the current reply from duplicate/stale events.

Original SDK results and raw provider JSON stay individually archived in PostgreSQL. The merged text is saved in the existing turn/model request evidence. New recognition operation records include `turn_assembly: confirmed-segments/1` and `turn_settle_ms: 1100`; existing records remain immutable. Progressive reply preparation, native recordings, audio queues, character assets and historical budgets are unchanged.

## Bounded verification

Default Go tests and native Speech SDK compilation pass. Focused race-enabled PostgreSQL tests use identity-verified disposable databases with external networking blocked. They reproduce the two-fragment example, wait beyond the deadline during an active continuation, verify one model/speech request and two preserved original results, and confirm a later separate turn. Additional cases cover duplicate callbacks, English/Spanish punctuation, intended repetition, no-match guesses, microphone stop, recognition failure and disconnect. Existing burst/interruption and recognition-dedup checks also pass.

Five focused Chromium/WebKit fixture scenarios pass for stale final events, streaming, interruption and microphone behavior. Windows WebKit's actual Web Audio capture case is explicitly skipped; its state/UI cases pass. No paid speech, recognition, model or image request belongs to verification. Owner microphone/room and physical-device acceptance remain pending. Source, logs and browser evidence are retained in `.tools/recognition-turns-40/` with generic PostgreSQL archival and verified restoration.
