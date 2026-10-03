# Run History Summary

This package was extracted from a completed Sub2API v0.2.13 ARM64 upgrade and deployment. The reusable workflow was separated from one-time paths, account names, timestamps, and credentials.

## What succeeded

- Source was merged from the official mainline after checking release tag metadata and source `VERSION` separately.
- Existing custom changes and intentional deletions were retained.
- A Docker/Buildx build produced and verified a `linux/arm64` image.
- Code was pushed to the configured `origin` without rewriting history.
- The first deployment attempt stopped before service switch when relative Compose paths resolved from the remote home directory.
- After resolving paths relative to the deployment root, the image was checksum-verified, loaded remotely, and only the application service was recreated.
- Application health and public health passed.
- PostgreSQL, Redis, leaderboard, and configured data mounts remained in place.
- A later release dry run recovered from a transient GitHub TLS failure through bounded retries.
- The release process was then made resumable with a manifest, staged gates, read-only status, and non-blind recovery classification.

## Reusable observations

| Observation | Invariant | Non-goal |
| --- | --- | --- |
| Network operations failed transiently | Retry idempotent transport with exponential backoff and verify postconditions | Do not retry an uncertain service switch blindly |
| Official tag and deployable mainline had different `VERSION` contents | Record tag metadata and source metadata separately | Do not assume a tag's file is always the deployable source version |
| Remote Compose used relative files | Anchor remote paths to deployment root | Do not depend on SSH working directory |
| Direct Docker access was denied but authorized sudo worked | Test configured permission paths before deployment | Do not bypass or expand privileges |
| Health checks are necessary but insufficient | Verify app, preserved services, mounts, and public health | Do not claim success from `/health` alone |
| Client transport can fail after a remote command starts | Persist command result, inspect one fresh remote snapshot, and classify success/not-switched/uncertain | Do not infer rollback from a healthy old image |
| Source can change between stages | Bind the manifest to one commit and version and reject drift | Do not reuse a release manifest for a different source |
| Local Docker can be unavailable while production remains healthy | Keep `status` and `recover` independent of local Docker; block only build-dependent stages | Do not report a local build as verified without Docker evidence |
