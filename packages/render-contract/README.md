# Render contract

Dependency-free Go types shared by `syntelle/api` and `syntelle/render-worker`: session settings/defaults/validation, private control envelopes, signaling payloads and native visemes. No rendering, provider, database or transport implementation belongs here.

Both services resolve this module from the monorepo and copy it into their own image build. Exact JSON fixtures protect compatibility between independently built API and worker images. Preserve existing wire tags/defaults and review future contract changes for compatibility with both deployed versions.
