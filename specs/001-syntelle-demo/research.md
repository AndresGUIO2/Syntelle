# Providers and costs for the Syntelle demo

Prices checked October 15, 2026; TTS selection updated October 16, 2026. Amounts are in US dollars. Azure Speech is selected for TTS and visemes. Google remains the priority for the LLM and recognition.

## Proposed providers

| Component | First prototype candidate | Alternative or comparison |
| --- | --- | --- |
| Facial geometry and regions | MediaPipe Face Landmarker | Change only if it fails photograph test-set validation. |
| Image variants | Nano Banana 2.1 through Gemini | Nano Banana 2 Lite, comparing fidelity and identity with the same prompts. |
| Depth | Depth Anything Small | Evaluate V2 Small as a reproducible project version without attributing it to the owner's previous workflow. |
| Responses | Gemini 3.1 Flash-Lite | Gemini 3.5 Flash-Lite if tutoring and customer service improvements justify the cost. |
| Speech recognition | Streaming Google Cloud Speech-to-Text V2 | Gemini Transcribe Live for comparison on cost and provider simplification, subject to access and trials. |
| Generated speech | Azure Speech Neural TTS, selected by the owner | Specific English and Spanish voices to test. Google and Voxtral remain researched comparisons outside the initial path. |
| Visemes | Azure Speech events with identifiers and timing offsets, selected as the base path | Verify mapping to our poses and synchronization; no external extractor is required initially. |

Azure Speech is selected for synthesis; other rows retain their stated candidate status. No providers are connected, quality trials have not run, and no paid calls have been made.

Azure Speech supplies audio and visemes through one provider. The owner subsequently confirmed Azure Container Apps for Go and FastAPI while keeping Vue on Cloudflare. Go calls Speech as a separate managed service. Measure end-to-end latency, quality, and cost; TTS and hosting choices do not automatically select Azure recognition. Prices below retain the check date above.

## Whisper and Voxtral as additional candidates

Options researched October 16, 2026 following the owner's suggestion. Azure Speech is selected for speech generation; recognition remains open. These comparisons are not part of the initial TTS implementation.

| Option | Documented function | Fit to evaluate |
| --- | --- | --- |
| Whisper | Audio-to-text transcription. In the API, whisper-1 supports word or segment timestamps but not streaming transcription. | Recognition or alignment over generated audio; word timestamps are not visemes. It does not replace speech synthesis. |
| Voxtral Mini Transcribe Realtime | Incoming audio transcription through WebSocket. | Candidate for visitor listening, separate from TTS. |
| Voxtral TTS, voxtral-mini-tts-2603 | Streaming speech synthesis with English, Spanish, and other languages. | TTS candidate. The reviewed generation documentation did not establish viseme output; do not assume it solves lip synchronization. |

Sources: [OpenAI transcription documentation](https://developers.openai.com/api/docs/guides/speech-to-text), [Voxtral Realtime](https://docs.mistral.ai/studio/audio/speech_to_text/realtime_transcription), [Voxtral TTS](https://docs.mistral.ai/models/voxtral-tts-26-03), [Mistral speech generation](https://docs.mistral.ai/studio/audio/text_to_speech/speech).

Using Mistral's API would add another voice provider to the backend. Self-hosting is a separate decision: Voxtral TTS has 4 billion parameters and its weights use a CC BY-NC 4.0 license. Resolve product usage terms and measure resources, startup, cost, and latency before adding it. Do not assume a lightweight CPU service or provision a GPU. [Model announcement](https://mistral.ai/news/voxtral-tts/), [model card and weights license](https://docs.mistral.ai/models/voxtral-tts-26-03).

These comparisons do not imply self-hosting Whisper or Voxtral or replace Azure Speech as selected TTS. The model's voice-cloning capability is not included in MVP scope.

## What MediaPipe provides

Face Landmarker estimates 478 facial landmarks and 52 expression coefficients and can provide transformation matrices. Use these outputs to prepare regions, correspondences, and geometry. They describe analyzed images; the engine must turn TTS visemes into animation. Run detection during preparation and reuse results during conversation. [Google AI Edge documentation](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker).

## Compared synchronization options

Gemini TTS documentation describes text input and audio output. This plan does not assume native viseme output. [Gemini TTS guide](https://ai.google.dev/gemini-api/docs/speech-generation).

Google Cloud TTS can return timing associated with SSML marks. These mark timings do not establish a complete phonetic sequence. [SSML timing marks](https://docs.cloud.google.com/text-to-speech/docs/ssml).

Rhubarb Lip Sync was examined as an extractor for already generated audio chunks. It maps mouth shapes to times; its English recognizer and language-independent phonetic mode need separate evaluation. The project reports lower accuracy for phonetic mode. Processing recordings in chunks adds analysis and waiting time; this path is outside the selected initial TTS implementation. [Original project](https://github.com/DanielSWolf/rhubarb-lip-sync).

Azure delivers timed viseme events and supports viseme identifiers for English and Spanish. This is the selected MVP path; voices and the animation controller still require validation. [Viseme events](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/how-to-speech-synthesis-viseme), [supported languages](https://learn.microsoft.com/en-us/azure/ai-services/speech-service/language-support#viseme).

## Reference prices

Standard checked rates, excluding account credits or discounts:

| Service | Unit | Reference |
| --- | --- | --- |
| Gemini 3.1 Flash-Lite | One million input/output text tokens | US$0.25 / US$1.50. |
| Nano Banana 2.1 | 1K output image | US$0.0336; input US$1.50 and output text or reasoning US$7.50 per million tokens. |
| Nano Banana 2 Lite | 1K output image | US$0.0336; input US$0.25 per million tokens. |
| Gemini 3.8 Flash-Lite TTS | Output audio | US$0.0015 per ten seconds through December 31, 2026; US$0.003 afterward. Input text is additional. |
| Google Cloud Speech-to-Text V2 | Standard audio minute, first volume tier | US$0.016. |
| Google Cloud WaveNet | One million characters | US$4 after applicable free usage. |
| Azure Neural TTS S1, East US | One million characters | US$15. |
| Azure STT S1, East US | Audio hour | US$1. |

Gemini prices come from the [official table](https://ai.google.dev/gemini-api/docs/pricing). Cloud references come from [Speech-to-Text](https://cloud.google.com/speech-to-text/pricing) and [Text-to-Speech](https://cloud.google.com/text-to-speech/pricing). Azure rates came from the [public Azure Speech S1 East US pricing API](https://prices.azure.com/api/retail/prices?$filter=productName%20eq%20%27Azure%20Speech%27%20and%20armRegionName%20eq%20%27eastus%27%20and%20skuName%20eq%20%27S1%27), because the commercial page did not resolve price amounts.

Do not select the old `gemini-2.5-flash-image` identifier: the official table lists its shutdown on October 2, 2026.

## External comparison

Amazon Polly Neural costs US$16 per million characters for speech or Speech Marks. A marks request, which includes visemes, does not return audio, so both requests would need budgeting. Azure remains the more direct comparison for a combined viseme workflow. [Polly pricing](https://aws.amazon.com/polly/pricing/), [mark types](https://docs.aws.amazon.com/polly/latest/dg/using-speechmarks.html).

Groq was also checked: it lists Whisper Turbo at US$0.04 per hour. That rate alone does not establish support for open conversation, interruptions, or segmentation needs. Do not add another provider initially for that price difference. [Groq models](https://console.groq.com/docs/models).

## Estimation scenario

This comparison uses Google WaveNet and preserves earlier estimation assumptions. It does not represent the currently selected Azure Speech TTS. Recalculate the budget with measured voices, consumption, and resources from the current architecture. The owner has deferred the global amount.

This is a planning calculation with explicit assumptions, not a measurement. The final image batch is undecided.

- A new character requires 12 to 20 generated 1K images using Nano Banana 2.1.
- Each image request is assumed to consume 1,500 input tokens and 500 output text or reasoning tokens. Replace these assumptions with actual API usage.
- A conversation accumulates 6,000 LLM input tokens across all calls and 800 output tokens.
- Process six microphone minutes and generate three speech minutes, equivalent to 2,700 characters in this example. Open-microphone billing may include silence; do not assume only visitor speech is charged.
- Use Cloud STT V2 and WaveNet. Exclude discounts and free usage.

| Item | 12 images | 20 images |
| --- | ---: | ---: |
| Image output | 0.4032 | 0.6720 |
| Assumed generation input | 0.0270 | 0.0450 |
| Assumed generation text or reasoning | 0.0450 | 0.0750 |
| LLM per conversation | 0.0027 | 0.0027 |
| Recognition per conversation | 0.0960 | 0.0960 |
| Generated speech per conversation | 0.0108 | 0.0108 |
| Subtotal | 0.5847 | 0.9015 |
| Additional 20 percent reserve | 0.1169 | 0.1803 |
| Estimate per new character and one conversation | 0.7016 | 1.0818 |

In this scenario, ten new characters with one conversation each cost approximately US$7.02-10.82; one hundred cost US$70.16-108.18. An additional conversation reusing the character costs US$0.1095 before the reserve under the same assumptions.

Using Gemini Flash-Lite TTS while retaining Cloud STT would yield a conversation subtotal of approximately US$0.1261 through the end of 2026: replace speech cost with US$0.0274, assuming 800 TTS input tokens. This is an economic comparison, not the selected Azure implementation budget.

The 20 percent reserve is a planning allowance, not a guarantee against all failures. Hosting, CPU for depth and viseme extraction, temporary storage, transfer, taxes, and price changes are excluded. Monthly total is observed consumption plus selected infrastructure. No monthly budget is fixed and billing is not authorized yet.

## Pending configuration and trials

A Gemini key enables its integrations but does not automatically enable Cloud Speech-to-Text or Cloud Text-to-Speech. Those services require their own Google Cloud project configuration and authentication.

Before final selection, measure generated image identity and alignment, successful character cost including failures, audio-plus-viseme latency, understanding in both languages, and interruptions using mobile speakers. Depth Anything V2 Small is a specific depth trial candidate; its repository identifies the model under Apache 2.0. [Official repository](https://github.com/DepthAnything/Depth-Anything-V2#license).
