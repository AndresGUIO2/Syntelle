# Bounded speech articulation

## Current correction: link motion between sound-anchored arrivals

On October 23 the owner reported choppiness specifically in the mouth, not head
motion or blinks. Version 3's short incoming ramps followed by full-pose holds
created stop/start articulation. Reducing transition duration had accentuated that
pattern. This diagnosis concerns the mouth timeline, not a measured video drop.

`bounded-articulation/4` anchors ordinary pose arrivals 15 ms after their audio
events and contact/rest arrivals at the events. It spreads travel before those
arrivals, using up to 140 ms between ordinary poses, and up to 70 ms when entering
MBP/FV or crossing rest. The preceding target must be reached before the next
transition starts; contact targets retain a 15 ms hold. Sustained vowels and long
silences retain intentional holds. Sampling remains stateless for pause/seek,
audio/event bytes are unchanged, and the fleeting-target filter is unchanged.
The 140 ms travel cap does not mean reaching a target 140 ms after its sound.

At 30 Hz, the cached 4.25-second recording's 31 targets produce 18 stationary
spoken frame steps instead of 37 out of 127 sampled steps. The largest single
pose-weight step decreases from 0.926 to 0.689. These are deterministic timeline
measurements, not source-pixel motion, physical mouth speed, measured received
fps or owner visual approval. Short contact gestures remain deliberately brief.

Focused tests verify audio-anchored arrivals, continuous pose weights on both
sides of anchors, actual interval travel, preserved contact/silence/vowel holds
and history-independent sampling. Native/reference and adapter checks pass with
the updated 3,281-frame fixture. Evidence and prior sources are in
`.tools/viseme-flow-83/`; the preceding package, textures and version 3 evidence
remain unchanged. Generic PostgreSQL archive/restore preserves this separately.
No provider requests, asset regeneration or transport/gesture changes are made.
Owner speech review remains pending.

## Previous tuning: a small speed increase

The owner requested only a little more speed after version 2.
`bounded-articulation/3` reduces ordinary/contact transition caps from 80/45 ms
to 70/40 ms, approximately 12% shorter. The 20 ms visual anticipation and
60/35 ms fleeting-target thresholds remain unchanged. The ordinary midpoint is
15 ms after its event, compared with 20 ms in version 2. This is a bounded
animation timing adjustment, not a measured transport synchronization result.
Version 2 remains in `.tools/viseme-pacing-66/` and PostgreSQL snapshot 570;
new before/current records are `.tools/viseme-pacing-82/`. Owner visual review
remains pending. No provider calls or asset changes are involved.

The native code now lives in `services/render-worker/internal/nativeanimation`;
the earlier API paths below describe their historical location.

## Previous correction: responsive articulation

The owner found the first tuning too slow and visibly delayed. The current
`bounded-articulation/2` keeps the same fleeting-target filter, but reduces the
transition caps from 120/70 ms to 80/45 ms (ordinary/contact). A fixed 20 ms
visual anticipation moves the halfway point of a full ordinary transition from
60 ms after the event to 20 ms after it. Shorter transitions remain bounded by
the available span. This is a mouth-sampling adjustment, not a change to PCM,
transport timestamps, event records or playback speed. No per-frame smoothing
state accumulates delay.

The prior version and its fixture remain in `.tools/viseme-pacing-60/` and
PostgreSQL snapshot 559. New before/current evidence is
`.tools/viseme-pacing-66/`. Updated tests assert the 20 ms anticipation bound,
the new midpoint, arrival within 60 ms for ordinary gestures, preserved contact
targets, continuity at anticipated boundaries and history-independent sampling.
The cached target count stays at 31. The ordinary gesture midpoint shifts 40 ms
earlier relative to version 1; this mathematical bound is not a measured
end-to-end audiovisual offset. Owner visual acceptance remains pending.

## First tuning (superseded after owner feedback)

The owner reported intermittent mouth changes that looked too fast. The native
worker and TypeScript reference previously drove every mapped target to its full
pose with transitions as short as 15 ms (45 ms maximum for MBP and 75 ms otherwise).
Dense event sequences therefore produced repeated fast excursions.

`bounded-articulation/1` suppresses intermediate mapped spans shorter than 60 ms.
MBP lip closures and FV contacts use a smaller 35 ms threshold. The first mapped
target is retained; equal neighbors are merged after filtering. Native input events
and audio bytes are preserved. Retained targets keep their original audio times;
the filter never stretches the recording or accumulates playback delay.

Smoothstep interpolation now uses up to 120 ms, or 70 ms for MBP/FV, bounded by
the complete available target span. It reaches the preceding target before the
next pair begins. Pausing, seeking and resuming remain stateless with respect to
mouth sampling. The same policy is implemented in native Go and its TypeScript
reference; materials, geometry, mapping and expressions are unchanged.

These thresholds are a local animation tuning choice, not measured physiology or
proof of phonetic accuracy. Very short contacts below 35 ms can be suppressed.
Owner visual review is required to judge the pacing.

## Bounded checks and reproduction

The cached 4.25-second Ava recording contains 52 native events. Mapped targets,
including terminal rest, decrease from 37 to 31. Focused tests cover brief vowel
and rest suppression, repeated-pose merging, preserved 40 ms MBP/FV contacts,
continuous pair boundaries, silence and history-independent sampling. Existing
3,281-frame TypeScript/native and adapter parity checks use the updated reference.
The prior reference is retained unchanged under `.tools/viseme-pacing-60/before/`.

Re-export offline reference values with:

```powershell
.tools/node-v24.22.0-win-x64/node.exe services/render-worker/scripts/export-animation-fixtures.mjs
```

Run `go test ./internal/nativeanimation ./internal/nativeadapter` from
`services/api`. The normal native image build also checks rendering and media
packages with its actual codec tags. Frontend type checking and scoped lint pass.
No paid provider call, image generation, asset calibration, database budget change,
deployment or new performance benchmark is part of this correction.

Before/current sources, cached counts, runtime identities and archive verification
are retained in `.tools/viseme-pacing-60/`. Use the generic PostgreSQL archive;
historical experiment finalizers must not overwrite these records. Actual visual
acceptance and physical-device speech review remain pending.
