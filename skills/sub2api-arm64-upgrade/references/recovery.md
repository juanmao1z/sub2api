# Recovery Matrix

## Manifest missing or source changed

Run `prepare` first. A manifest is bound to one source commit, source version, image tag, and release name. If the source commit or version changes, create a new release instead of reusing the old manifest.

## Official source fetch fails

Retry `git fetch` with bounded exponential backoff. After a successful-looking retry, verify the expected ref and source `VERSION`. If all attempts fail, stop before merge and report the transport error.

Do not switch remotes, disable TLS verification, force-push, or infer that a fetch succeeded from a previous local object.

## Merge conflicts

The official source may modify files that the project intentionally deleted or customized. Treat unmerged paths as a semantic review gate. Preserve intentional local deletions and custom behavior, then run project checks again. Do not use a blanket `ours` or `theirs` resolution.

## Build or image failure

Use the configured Docker/Buildx build as the validation boundary. Fix only the diagnosed source or toolchain issue, rerun the target build, and inspect the resulting image architecture. Do not release a partially built or architecture-mismatched image.

If a local Docker daemon is unavailable, `build` is blocked. This does not invalidate read-only manifest inspection or remote `status`/`recover` checks.

## Version mismatch

Keep these facts separate:

- official release tag and its metadata;
- verified official source commit;
- source `VERSION` in the tree being released;
- expected deployment version.

A release is blocked unless the configured verification fields and the source file agree. Update the fact source only from checked repository evidence.

## GitHub TLS or push interruption

Retry only idempotent `fetch`, `push`, and reference verification with bounded backoff. After a successful push, compare local `HEAD` and `origin/main`. If the operation's final state is unknown, query the remote ref before repeating it.

## SSH or SCP interruption

Retry SSH preflight and archive upload with bounded timeouts. Verify the remote archive checksum and loaded image architecture. A failed SCP does not authorize a blind service switch.

## Service switch ambiguity

`apply` records the command result before checking the remote postcondition. If the postcondition cannot be verified, the manifest state is `uncertain`; use `status` or `recover` before any further apply.

`recover` obtains one remote snapshot and classifies it as:

- `success`: target image, app health, preserved services, mount evidence, and public health pass;
- `not_switched`: the old image is healthy and the target image is not active;
- `uncertain`: remote status or required health evidence is unavailable or contradictory.

A healthy old image is not proof that an application rollback completed. Do not blindly rerun the full service switch.

## Compose path failure

Resolve relative Compose filenames against the configured deployment root. Do not rely on the SSH session's current directory or the remote user's home directory.

## Remote Docker permission failure

Use only already-authorized non-interactive `sudo` when the configured deployment path requires it. If neither direct Docker access nor authorized sudo works, stop. Do not add the user to groups, change socket permissions, or escalate interactively as part of a deployment.

## Health failure after switch

Restore the backed-up application override, recreate only the application service, wait for application health, and verify public health again. Report rollback status explicitly. PostgreSQL, Redis, leaderboard, and data mounts must not be part of rollback commands.
