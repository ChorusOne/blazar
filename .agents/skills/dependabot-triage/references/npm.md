<!--
DO NOT EDIT DIRECTLY
The reference copy of this skill lives in the conductor repository.
Make changes there first, then copy them here.
-->

# Node.js (npm)

## Manifests and scope

Alerts are raised on the lockfile (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`)
and `manifest_path` is the lockfile's path, so the project directory is its parent.
Do not read scope from the alert; `npm ls <package> --omit dev` below is the check.

A lockfile can hold several versions of one package, hoisted at `node_modules/<package>`
and nested under a parent (`node_modules/tsx/node_modules/esbuild`). Each vulnerable
copy gets its own alert on the same lockfile. A bump that moves the hoisted copy leaves
the nested one in place, so check `npm ls <package>` after the bump.

## Who depends on it

From the project directory:

```
npm explain <package>
npm ls <package> --omit dev
```

`npm explain` prints every path from a direct dependency to the package, with versions.
`npm ls --omit dev` prints nothing when the package is only reached through
`devDependencies`, which is the dev-dependency scope. The scope is the project
directory, as dependency or dev-dependency.

## Call sites

```
git grep -n -E "from ['\"]<package>[/'\"]|require\(['\"]<package>[/'\"]" -- '<project>'
```

`node_modules` is not tracked, so `git grep` only searches our code. Build tools
(bundlers, markdown renderers in a static site generator) have no call sites: they run
at build time on files we write, so their vulnerable code is reached only when they
process input we do not control.

## Bump

Direct dependency:

```
npm install <package>@<patched-version>
```

Add `--save-dev` when it is in `devDependencies`. This rewrites both `package.json` and
the lockfile.

Transitive dependency: `npm update <package>` moves it when every parent's range admits
the patched version. Otherwise add an override to the root `package.json` and run
`npm install`:

```json
"overrides": {
  "<package>": "<patched-version>"
}
```

Overrides are only read from the root `package.json`. To pin one nested copy, nest the
key under its parent: `"<parent>": { "<package>": "<patched-version>" }`. An override is
a lasting pin that hides future parent updates; say in the PR body which parent release
makes it removable.

## Outputs that look like failures

- `npm update <package>` reporting `up to date` while the lockfile still has the old
  version: a parent's range caps it. Use an override.
- A lockfile diff that touches far more than the package and its parents: the local npm
  rewrote the lockfile in a newer format. Use the npm version CI uses (`engines` in
  `package.json`, or `setup-node` in the workflow) and redo the bump.

## Default verification

```
npm ci
npm run build
```

Then `npm test` and `npm run lint` when those scripts exist in `package.json` and CI
runs them.
