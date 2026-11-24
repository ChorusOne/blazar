---
# DO NOT EDIT DIRECTLY
# The reference copy of this skill lives in the conductor repository.
# Make changes there first, then copy them here.
#
name: dependabot-triage
description: Resolve open Dependabot alerts for the current repository (Rust, npm, Go or Python). For each advisory decide whether the vulnerable code is reachable, bump the dependency when it is (or when the bump is cheap), verify with the project's CI commands, open a PR, and record every advisory left open, whether ignored or blocked, in dependabot-exceptions.yaml. Use this only when the user asks to triage, resolve, fix, or clean up Dependabot alerts or the Security tab. A question about a CVE, an advisory, or whether some code is affected is not a request to change anything; answer it directly without this skill.
---

# Dependabot triage

The goal is that the repository is not vulnerable.
Dependabot finds the advisories. This skill decides what to do with each one, does it,
and leaves a record that a future run can check.

Three outcomes exist per advisory:

- **Bump**: the fix version is applied, CI passes, a PR is opened.
- **Ignored**: the vulnerable code is provably not reached *and* the bump is not cheap.
- **Blocked**: a bump is needed but something prevents it (no patched version, a
  breaking API, a transitive pin). This is the vulnerable case: the repository *is*
  vulnerable until the blocker goes away.

Ignored and blocked advisories are both written to `dependabot-exceptions.yaml` so the
decision survives this session and the next run knows what to re-check. A blocked entry
is the more important of the two: it is the list of known exposures, and every run must
try again to clear it.

Cheap bumps are never skipped. An ignore entry be must checked every time the code around that dependency changes.
A one-line version bump that passes CI costs less than that. Only write an ignore entry when the bump would be real work.

Never dismiss alerts yourself: print the exact `gh api` PATCH commands in the PR body for the reviewer to run
after merge, so a human approves the "not vulnerable" claim before GitHub records it.

## Step 1: collect the open alerts

```
gh api 'repos/{owner}/{repo}/dependabot/alerts?state=open&per_page=100' --paginate
```

Take `{owner}/{repo}` from `gh repo view --json nameWithOwner`. Group alerts by
`security_advisory.ghsa_id`, not by CVE: every alert carries a GHSA, only some carry a
CVE, and one advisory produces several alerts (one per affected package and manifest).

Per group keep:

- `number` and `html_url` of every alert
- `dependency.package.ecosystem` (`rust`, `pip`, `npm`, `go`)
- `dependency.package.name` of each affected package
- `dependency.manifest_path` and `dependency.scope` of each alert
- `security_vulnerability.vulnerable_version_range` and `first_patched_version`

`dependency.scope` is `runtime` or `development`. Whether it can be trusted depends on
which manifest the alert is on; the ecosystem reference says which alerts carry a
meaningful scope.

The commands for Steps 3, 5 and 6 differ per ecosystem and live in
`references/<ecosystem>.md`: `rust`, `npm`, `go`, `pip`. Read the file for every
ecosystem that appears in the alerts before going on.

If the user named specific alerts, packages or an ecosystem, filter the groups now.

## Step 2: check the exceptions file

Read `dependabot-exceptions.yaml` at the repository root if it exists. For each open
group whose GHSA is a key in the file:

- If the locked version is now outside `vulnerable_versions`, the entry is stale. Delete
  it and treat the group as resolved; the alert will close on its own.
- If the scope recorded in the entry no longer matches what Step 3 finds (a new component
  depends on the package, or a dev-dependency became a runtime one), the assessment is
  void. Delete the entry and re-assess from scratch.
- `status: ignored`: re-read the `reason` and confirm it still holds against the current
  code with one grep. If it does, skip the group. If it does not, delete the entry and
  re-assess.
- `status: blocked`: check whether the blocker still stands. A patched version may have
  appeared, the locked parent package may have released, the breaking API may have been
  migrated by someone else. If anything changed, retry the bump from Step 6. If the
  bump succeeds, delete the entry. If it still fails, update `reason` only when the
  blocker changed.

A key in the file with no open alert is stale, whatever its status: someone fixed the
advisory with an unrelated bump. Delete it.

A stale or void entry is the whole point of the file: it is how "not vulnerable yet"
becomes an alarm instead of a forgotten note, and how "cannot fix yet" gets retried
instead of forgotten.

## Step 3: find where the dependency is used

Run the dependents command from the ecosystem reference. It names every component (a
workspace member, a project directory, a Go module) that depends on the package and
whether the edge is a dev-dependency. Those components are the `scope`. Then run the
call-site grep from the reference against those components.

## Step 4: assess reachability

Read the advisory:

```
gh api advisories/<GHSA> --jq '{summary, description, vulnerabilities}'
```

Identify the vulnerable code: a function, a module (server versus client), a feature
flag, or an input-handling path. `vulnerabilities[].vulnerable_functions` is often empty,
so read `description` and the linked upstream advisory when needed.

Then decide, from the call sites found in Step 3:

- **Reached**: our code calls the vulnerable function, enables the feature, or feeds the
  vulnerable path with input we do not control.
- **Not reached**: none of the call sites can enter the vulnerable code. Typical cases:
  server-side code in a library we only use as a client, a feature flag we do not enable,
  a dev-dependency that never ships in a binary.
- **Unclear**: the advisory is vague or the call graph is too deep to follow.

Treat *unclear* as *reached*. The goal is not being vulnerable, and a bump is cheaper
than a wrong "not reached".

A dev-only dependency is "not reached" for shipped binaries, but still bump it when the
bump is cheap.

## Step 5: decide

| Reachability | Bump is cheap | Action |
|---|---|---|
| reached or unclear | any | bump; if the bump fails, blocked entry |
| not reached | yes | bump |
| not reached | no | ignored entry |
| not in the build graph | any | remove the requirement |

A bump is **cheap** when all of these hold:

- A patched version exists.
- The package is a direct dependency of a component, or a transitive one whose patched
  version satisfies the parent's version requirement so the lockfile-only update from
  the reference moves it.
- The patched version is within the semver range the manifest already declares, or the
  change log shows no breaking change on the APIs we call.
- The verification commands in Step 6 pass without code changes.

Anything else (major version jump with API changes, a transitive pin that needs the
parent updated first, a vendored copy) is **not cheap**.

## Step 6: bump and verify

Bump every advisory of the same package in one commit. All bumps of one ecosystem go in
one PR, one commit per package: sibling lockfile changes conflict when they land through
separate PRs, and one PR is what a reviewer can read. Each ecosystem gets its own PR,
since it has its own lockfile, its own verification commands and often its own reviewer.
Start every change from an up-to-date default branch, never from whatever branch happens
to be checked out: alerts are computed against the default branch, and a feature branch
may already carry a bump that is not merged.

Run the bump command from the ecosystem reference. Before calling a bump blocked, check
the reference's list of outputs that look like failures but are not.

Verify with the same commands CI runs: read the workflows under `.github/workflows` and
run those commands locally for the affected component. When CI runs nothing for that
component, use the default verification commands in the ecosystem reference.

If verification fails and the fix is a small, obvious API adjustment (a renamed method,
a changed argument), make it and verify again. If it needs more than that, revert and
write a blocked entry with the error output summarised in `reason`.

## Step 7: write exception entries

`dependabot-exceptions.yaml` at the repository root, one map entry per advisory, keyed
by GHSA. Keep the keys and their order exactly:

```yaml
# Dependabot alerts left open on purpose. One entry per advisory.
# ignored: the vulnerable code is not reached and the bump is not cheap. The reason
#   states the condition that keeps us safe, so a later change can be checked against it.
# blocked: the vulnerable code is reached and the bump is not possible yet. The reason
#   states the blocker, so a later run knows what to check before retrying.
GHSA-xxxx-xxxx-xxxx:
  status: ignored
  cve: CVE-2026-00000            # omit the key when the advisory has no CVE
  alerts:
    - https://github.com/<owner>/<repo>/security/dependabot/62
    - https://github.com/<owner>/<repo>/security/dependabot/63
  packages: [russh, russh-cryptovec]
  vulnerable_versions: "<= 0.60.2"
  scope:
    conductor-ssh: dev-dependency   # component from Step 3, then dependency or dev-dependency
  vulnerable_code: CryptoVec growth on inbound packets handled in server mode
  justification: not_used
  reason: conductor-ssh constructs only russh::client sessions, server mode is never entered

GHSA-yyyy-yyyy-yyyy:
  status: blocked
  cve: CVE-2026-00001
  alerts:
    - https://github.com/<owner>/<repo>/security/dependabot/23
  packages: [old-crate]
  vulnerable_versions: "<= 0.1.8"
  scope:
    straw: dependency
  vulnerable_code: type confusion in Error::downcast
  reason: no patched version exists, old-crate is unmaintained and reaches us only through parent-crate 1.2.0, which has no release without it
```

Field rules:

- `status` is `ignored` or `blocked`.
- `justification` only exists on ignored entries. It is one of the values the Dependabot
  API accepts for a dismissal: `not_used` (vulnerable code not reached),
  `tolerable_risk` (reached but accepted), `inaccurate` (advisory does not apply, for
  example a wrong version range). `fix_started` and `no_bandwidth` are alert states,
  never ignore decisions. Blocked entries have no justification because their alerts
  stay open: the Security tab must keep showing a known exposure.
- `reason` on an ignored entry is a condition a later reader can falsify with a grep,
  not a verdict. "Not affected" tells a future run nothing. "Server mode is never
  entered" tells it what to look for. On a blocked entry it is the blocker, concrete
  enough to retry: which package caps the version, which API broke, or that no patched
  version exists.
- Do not add fields. Author and date come from git blame.

Exception entries go in the PR of their ecosystem. A bump is checked by CI and the
lockfile diff; an entry is a claim about the codebase that needs reading. One commit per
entry, after the bumps, lets a reviewer read the two apart and drop a single claim.

## Step 8: open the PR

One PR per ecosystem, on branch `dependabot/<ecosystem>-<date>`.

Commits, one per package, each applied on top of the previous one so the lockfile never
conflicts: the manifest requirement and the lockfile, nothing else. A code change that
makes a bump compile is its own commit right after that bump, limited to the files that
call the package. Changes to `dependabot-exceptions.yaml` are one commit per entry at the end. The
split is for review: the reader sees the lockfile noise, the real change and the claims
apart.

Commit subjects and PR titles follow the repository's convention; read
`git log --oneline -20` first. In the component-then-imperative-sentence style:

```
Conductor-ssh: Bump russh to 0.65.0
```

```
Conductor-ssh: Adapt the SSH mock to russh 0.65 handler traits
```

```
Ignore GHSA-xxxx-xxxx-xxxx, server mode is not used
```

```
Record GHSA-yyyy-yyyy-yyyy as blocked, old-crate has no patched version
```

PR body, exact template. It is written for the reviewer. It opens with a fixed
paragraph that says an AI agent wrote it with this skill
and what the skill does in plain words, so a reader knows what to trust and what to
check. Packages are grouped under a severity header, from critical down to
low, each package placed by the highest severity among its advisories. Under a package,
one bullet per advisory: the GHSA linked to the GitHub advisory page, the CVE after it
when one exists, then the severity, a short summary and a link to the Dependabot alert.
Then the prose for that package:

```
This PR was created by an AI agent with the dependabot-triage skill. The skill reads
the open Dependabot alerts for this repository, checks for each advisory whether the
vulnerable code can actually run in our binaries, then bumps the dependency or removes
it when nothing uses it, and runs the same checks as CI on the result.

## High

### <package>
- [GHSA-...](https://github.com/advisories/GHSA-...) CVE-... (high): <summary>, [alert <n>](<alert url>)
- [GHSA-...](https://github.com/advisories/GHSA-...) (medium): <summary>, [alert <n>](<alert url>)

<two or three sentences: where the package is used, whether the vulnerable code is
reached, why this is a bump, an ignore, or a blocked entry>

## Medium

### <next package, same shape>

After merge, dismiss the alerts so the Security tab matches the repository:

gh api -X PATCH repos/<owner>/<repo>/dependabot/alerts/<n> -f state=dismissed -f dismissed_reason=<justification> -f dismissed_comment='<reason, at most 280 characters>'
```

The "After merge" paragraph only appears when the PR adds an ignored entry. Bumped
alerts close automatically once Dependabot rescans the default branch. Blocked alerts
stay open on purpose.

## Step 9: report

End with one table, one row per advisory group, no prose before it:

| GHSA | CVE | package | scope | reachability | action | PR |
|---|---|---|---|---|---|---|

`action` is `bumped to <version>`, `removed`, `ignored`, `blocked: <one line why>`,
`stale entry removed`, or `still blocked`. Then list the blocked groups with the error output, if
any. Blocked groups are the known exposures, so they come first in that list.
