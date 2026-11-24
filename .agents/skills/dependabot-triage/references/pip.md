<!--
DO NOT EDIT DIRECTLY
The reference copy of this skill lives in the conductor repository.
Make changes there first, then copy them here.
-->

# Python (pip, pipenv)

## Manifests and scope

Alerts are raised on the file that pins the version: `requirements.txt`, `Pipfile`,
`Pipfile.lock`. A Pipfile project gets twin alerts on `Pipfile` and `Pipfile.lock` for a
direct dependency and a `Pipfile.lock` alert alone for a transitive one.
`dependency.scope` is `development` for `[dev-packages]` on both Pipfile alerts. On a
`requirements.txt` alert it is always `runtime` because the format has no dev kind.

Alerts keep the name's case from the manifest, so `GitPython`, `Gitpython` and
`gitpython` are one package. Compare names case-insensitively with `-` and `_` equal.

Two `requirements.txt` layouts exist and they bump differently:

- Direct dependencies only, paired with a `requirements.frozen` written by `pip freeze`.
  The alert is on `requirements.txt`, CI and the README install `requirements.frozen`,
  so both files must change. A transitive package has a pin only in the frozen file.
- Freeze output committed as `requirements.txt`. Every transitive package is pinned and
  gets its own alert, and bumping one pin may require moving the pins it depends on.

## Who depends on it

The scope is the directory holding the manifest, as dependency or dev-dependency. For
a transitive package, find the direct dependency that pulls it in:

```
<venv>/bin/pip show <package>
pipenv graph --reverse
```

`pip show` prints a `Required-by:` line. Run `pipenv` from the directory holding the
`Pipfile`.

## Call sites

```
git grep -n -E '^\s*(import|from) <module>\b' -- '<dir>/*.py'
```

`<module>` is the import name, which can differ from the package name (`GitPython` is
imported as `git`, `PyYAML` as `yaml`). `pip show -f <package>` lists the top-level
module. Tools (`black`, `mypy`, `pytest`, `yamllint`, `ansible-lint`) are never
imported: their vulnerable code is reached only when they run on input we do not
control, and in CI they run on our own files.

## Bump

`requirements.txt`: edit the pin. With a `requirements.frozen` pair, edit the pin in
both files. A one-line edit to the frozen file is enough when the new version's own
requirements are still satisfied; the verification step below reports a conflict when
they are not, and the conflicting pins move in the same commit.

`Pipfile`:

```
pipenv upgrade <package>==<patched-version>
```

This resolves and rewrites `Pipfile` and `Pipfile.lock` without installing. For a
transitive package it adds the package to `Pipfile` as a direct dependency, which is the
pipenv equivalent of an override; say so in the PR body. `pipenv lock` instead
re-resolves every package and produces a diff nobody can review, so avoid it.

## Outputs that look like failures

- The pipenv `Courtesy Notice` about running inside a virtual environment: pipenv will
  use the active venv instead of its own. Set `PIPENV_IGNORE_VIRTUALENVS=1`.
- pip `ResolutionImpossible` after editing a frozen file: the bumped package needs a
  newer version of a pin below it. The message names the pin; move it too.
- pipenv refusing the local interpreter: `[requires] python_version` in the `Pipfile`
  differs from the default `python`. Pass `--python <version>`.

## Default verification

In a fresh virtual environment created outside the repository so it cannot end up in
the commit:

```
python -m venv <tmpdir>/venv
<tmpdir>/venv/bin/pip install -r <requirements.frozen or requirements.txt>
<tmpdir>/venv/bin/pip check
<tmpdir>/venv/bin/python -m py_compile <each script in scope>
```

For a `Pipfile`, `pipenv sync --dev` then `pipenv run pytest` when tests exist. Then
whatever CI runs on that directory, typically a `Makefile` target or a lint script.
