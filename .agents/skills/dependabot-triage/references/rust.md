<!--
DO NOT EDIT DIRECTLY
The reference copy of this skill lives in the conductor repository.
Make changes there first, then copy them here.
-->

# Rust (cargo)

## Manifests and scope

Every locked vulnerable version raises an alert on `Cargo.lock`. A direct dependency
raises a second alert on the member's `Cargo.toml`. `dependency.scope` is only
meaningful on the `Cargo.toml` alert: the `Cargo.lock` alert always says `runtime`
because a lockfile has no edge kinds. A transitive dependency has no manifest alert, so
its scope comes from the dependents command below, not from the alert.

A repository with several lockfiles (`Cargo.lock`, `aptos/Cargo.lock`) gets one alert
per lockfile. Bump each lockfile.

## Who depends on it

From the workspace root:

```
cargo tree --workspace -i <package> --depth 1 --target x86_64-unknown-linux-gnu -e normal
```

The target is the platform the Dockerfiles build for. Without it cargo tree resolves
for the machine running it and hides Linux-only edges such as openssl behind native-tls.

The members listed are the runtime dependents. Run the same command without `-e normal`
and every member that only appears there depends on the package through a dev or build
edge. Together they are the scope. Ignore the `[dev-dependencies]` and
`[build-dependencies]` lines in the second output: they are indented under a member
when they describe that member's own dependents, which `--depth 1` cuts off. With
several locked versions, `-i <package>@<version>` selects one.

Empty output from both runs means the package is in `Cargo.lock` but not in the build graph: a
platform-gated dependency, or a requirement nothing uses any more. Check the
`Cargo.toml` that declares it; removing a dead requirement closes the alert.

## Call sites

```
git grep -n '<crate>::' -- '<member>/*.rs'
```

`<crate>` is the package name with hyphens turned into underscores, which is how Rust
code refers to it (`russh-cryptovec` is used as `russh_cryptovec::`).

## Bump

```
cargo update -p <package> --precise <patched-version>
```

If the manifest's requirement does not admit the patched version, edit the requirement
in the member's `Cargo.toml` first, then run `cargo update -p <package>`.

## Outputs that look like failures

- `Unchanged <package> (available: <version>)` means another locked crate caps it. Rerun
  with `--precise <version>`: the error then names the blocker, which is the crate to
  bump first.
- `--precise` takes one package per invocation. With several the error is a misleading
  "no matching package".
- Cargo skips versions whose `rust-version` is above the toolchain and resolves an older
  one silently. If the bump lands below the patched version, compare the patched
  version's `rust-version` with `rust-toolchain.toml`. A toolchain bump is the user's
  decision: report it, do not force it.

## Default verification

```
cargo fmt --check
cargo clippy --tests -- -D warnings --no-deps
cargo test
cargo build
```
