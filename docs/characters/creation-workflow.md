# Private character preparation

The account upload queue is consumed by `app.workflows.characters.worker`. It builds a private native package from the submitted image; it never substitutes Syntelle or uses a different portrait when preparation fails. The API remains responsible for account ownership, admission, idempotency and ready-only playback.

## Run the local consumer

From `services/processor`, using the existing preparation environment:

```powershell
.venv/Scripts/python.exe -m app.workflows.characters.worker `
  --packages <private-native-package-directory> `
  --scratch <separate-private-temporary-directory> `
  --checker <trusted-phase122-or-later-render-frame.exe>
```

The consumer uses the personal `SYNTELLE_DATABASE_URL` from the environment or repository secret configuration. Both directories must be outside web-served assets and separate from each other. Each database needs its own package directory: deletion checks use only that database's ownership records, so an isolated test must never share the demo's private package directory. Configure the native worker to resolve approved package hashes from the same package directory. `--once --id char_<32-lowercase-hex>` processes one selected job for a bounded local check. A regular consumer polls the durable queue every two seconds and recovers after a database disconnect.

Without `--approval`, exact source-bound cached preparations remain available. An unknown portrait fails with `generation_not_enabled`; it is never reported ready or replaced with a known face. Complete previously published packages from the same preparation revision are reused after source/hash and native-render validation, including portraits absent from the development catalog.

## Optional finite generation campaign

An operator-approved campaign permits ordinary account uploads to run without per-job intervention until the campaign expires or its allowance is consumed. It is separate from every historical experiment. For example, **only after explicit authorization for one new portrait**, pass a private policy file with `--approval`:

```json
{
  "version": "character-creation-campaign/1",
  "campaign_id": "owner-approved-new-portrait-001",
  "maximum_characters": 1,
  "limits": { "image": 9, "analysis": 9 },
  "expires_at": "<explicit future UTC timestamp>"
}
```

This is a template, not an authorization or runnable policy. The full remaining allowance must fit before any new call. A database lock serializes campaign spending across consumers; its immutable binding prevents budget resets. Each attempt is reserved in PostgreSQL before the call. Provider errors stop the campaign, and uncertain attempts are not retried. There are no corrections, substitutions or fallback requests.

The existing local models must be cached with valid pins before any generation: MediaPipe landmarks and segmentation, Depth Anything V2 Small, and ViTMatte. The existing personal generation credentials remain server-side. A portrait must have one detectible near-frontal face, an accepted JPG/PNG/WebP format, no more than 4,194,304 pixels or 4,096 pixels on either side, and an aspect ratio within 2% of 1:1, 3:4, 4:3, 9:16 or 16:9. Rejections before generation spend no requests.

The complete new-source path uses eight original-bound mouth images, one closed-eye image and at most nine semantic oral analyses. It measures and registers the generated observations, prepares oral materials, foreground/depth, head projection, gaze and expression fields, then exports private native resources. Generated requests preserve the original portrait bytes; there are no manual character masks or placement overrides. Geometry, attachment and source-binding failures reject the package.

Ready requires all nine poses and 72 directed mouth pairs, valid projection/gaze/expression fields, complete content-hash-checked resources, and successful native Rest/A/FV/MBP frame rendering with actual articulation differences. These technical checks do not establish perceptual quality for arbitrary future portraits. The basic prepared package has no qualified Syntelle-specific laughter or advanced emotional companions; runtime capabilities must reflect its actual resources.

## Retention and recovery

Uploads and complete provider request/response bodies are private. The complete request is committed before sending; original response bytes and status are recorded separately. These bodies expire no later than the upload's 24-hour deadline and are erased on deletion. The durable budget ledger retains hashes, counts and status only. Temporary working directories are removed after completion/failure and recovered after an interrupted process. Published native packages remain private while referenced by a live character and are removed after their final owner reference is deleted. Never archive customer photo bodies into permanent immutable experiment blobs.

The worker marks an abandoned `processing` job failed instead of replaying potentially billed work. Received provider bodies are spooled privately before their database write; after a database outage, the next consumer recovers these bytes before deleting the working directory, without replaying generation. HTTP200 responses containing a provider error or an unexpected JSON type also stop the campaign. Tab closure does not affect queue ownership. A package is published with an atomic rename on its destination filesystem; the ready record is committed only after source/hash/native rendering validation.

## Offline verification

`tests/test_character_creation_worker.py` covers the real queue and native export using an isolated PostgreSQL database, account-source binding, exact prepared reuse, interrupted-job recovery, deletion/expiry, full input persistence before provider invocation, retained provider rejection (including HTTP200 error bodies), response recovery after a database failure, no retry, and campaign whole-job admission. Eight focused cases pass. Set `SYNTELLE_CHARACTER_CREATION_TEST_DSN` to a local test-admin connection and `SYNTELLE_NATIVE_FRAME_CHECKER` to the trusted frame checker. The fixture creates and verifies a separate database before mutation and drops only that database afterward.

The distinct Café source for an account upload check is `.tools/semantic-oral-06/corpus/cafe/original.png`, SHA-256 `056835109482e3d3542d2b1ed394c132664abab656bf510e2af8e35e47940e01`. Its exact cached package is `779546eeaeee7a714a14438aa29720f6b4f05d365e596d6f3bc42aecc313c446`. The Woodland cache has its own immutable source and package in `creation-source-cache.json`.

The recorded replay in `.tools/character-creation-review/replay_generation.py` recomputes the full new-source preparation from 18 byte-matching historical responses, with zero new provider calls. It produced package `38222e53b34ffd3b54d38d08df1c37ecdf1d6ca6dceb80b3a804c4679e9c6058`; exported identity currently includes preparation paths, so this differs from the exact cached export. The frontal native Rest/A frames show the uploaded Café identity and distinct coherent articulation. This is a replay of an existing development portrait, not an unseen-portrait qualification or proof of general visual acceptance.
