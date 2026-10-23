# Character assets and animation reference

The local example supplied by the owner helps explain the prepared image set and robotic movement they want to improve. Files and code were inspected without running the animation. The potential causes of rigidity below are hypotheses to test in the prototype.

## Reference usage

The owner identified this example as copyrighted material and requested particular care. Its use in Syntelle is limited to analysis: do not copy images, maps, variants, or code into the new project or public demo. Generate owned or authorized resources through the Nano Banana workflow.

The inspection did not modify the reference project. Its illustrated appearance does not change the agreed MVP scope: frontal photographs of one person with a clearly visible face.

The owner subsequently selected a similar semi-realistic illustrated treatment for Syntelle's own sample character, rejecting the photorealistic human generated in the UI mockups. This authorizes a broad style direction for an original owned or authorized sample, not reuse of this reference's identity or assets. Visitor image compatibility remains a separate validation requirement.

## Local location

Assets: `E:\Carpetas\Documentos\01-MisProyectos\pitra\EchoHostAI-OrgLensAI\apps\frontend\src\assets\talking-avatar`.

Inspected animation controller: `apps/frontend/src/components/talking-avatar/` within that external project.

## Observed inventory

| Resource | Count | Observation |
| --- | --- | --- |
| Base image | 1 | Portrait with the mouth at rest. |
| Mouth variants | 8 | Groups A, EI, O, U, FV, LDTN, SZ, and CHSHR. |
| Blink variants | 2 | Partially closed and closed eyes. |
| Depth maps | 9 | One base map and one for each mouth variant. |

The inspected visual samples share an 896 by 1200 pixel canvas. Variants are transparent overlays on the base image. The code handles MBP, but the recorded asset set has no independent image for that lip closure.

The subsequently supplied prompts include an MBP description but no LDTN description. The workflow and this discrepancy are documented in [Image preparation with Nano Banana](character-preparation.md).

## Behavior observed in code

- The mouth follows audio playback time and Azure viseme events.
- Transitions between open poses select one pose and then the other; they do not geometrically deform one mouth into the next.
- The controller filters short events, prioritizes vowels, and removes MBP events except for some final closures.
- During some sustained vowels, it inserts alternate poses, including fallback alternatives when no suitable nearby pose is found.
- Blinking switches between two images through opacity changes.
- Autonomous motion, parallax, and breathing already exist. The starting point is not a completely motionless character.

## Hypotheses to test

Direct pose replacement may produce jumps even when each change is timed to audio. Intermediate variants or controlled deformation could smooth them; verify that the result produces neither double mouths nor identity changes.

Removing MBP closures and inserting alternate poses during sustained sounds may weaken lip-to-speech correspondence. Evaluate naturalness and articulation together, especially in English.

The inspected mouth layers include parts of the jaw and neck. Preserve alignment and avoid lighting differences or visible depth changes at their boundaries.

## Proposed asset expansion

Evaluate partial-opening variants for existing mouths, a dedicated M/B/P closure, and additional blink phases. These are test proposals, not an agreed image count.

The final set should cover transitions that visibly improve animation, with aligned resources optimized for mobile. Select image count and transition mechanism together. Compare fast phrases, sustained vowels, closures, and pauses, using audio as the timing reference.
