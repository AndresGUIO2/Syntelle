# Private character performance profiles

The default directory contains no prepared resources. Set
`SYNTELLE_PERFORMANCE_PROFILE_DIR` to a private prepared directory and
`SYNTELLE_PERFORMANCE_PROFILE` to its `/opt/syntelle-performance/<file>.json`
container path to enable a reviewed, package-bound profile. The directory is
mounted read-only and is never served to the browser.

Create profiles with `app.workflows.motion.prepare_performance_profile` after
composition and cached-audio playback review. Owner acceptance is recorded
separately. No profile keeps the preceding character capability behavior.
