<!--
DO NOT EDIT DIRECTLY
The reference copy of this skill lives in the conductor repository.
Make changes there first, then copy them here.
-->

# Go (modules)

## Manifests and scope

Alerts are raised on `go.mod`. Go has no dev-dependency kind, so `dependency.scope` is
always `runtime`. A module used only by tests is found with the call-site grep below:
every call site is in a `_test.go` file.

## Who depends on it

From the module root:

```
go mod why -m <module>
go mod graph | grep ' <module>@'
```

`go mod why` prints the shortest import path from the main module to the module. The
note `(main module does not need module ...)` means the module is in `go.sum` but not in
the build graph; `go mod tidy` removes it and closes the alert. `go mod graph` lists
every parent that requires the module, with the version each one asks for.

The scope is the Go module (one per `go.mod`), and dependency or dev-dependency from
where the call sites are.

## Call sites

```
git grep -n '"<module>' -- '*.go'
```

Import paths start with the module path, so this matches every package of the module.

## Reachability

Run `govulncheck` before reading the advisory by hand:

```
go run golang.org/x/vuln/cmd/govulncheck@latest -show traces ./...
```

It does symbol-level reachability against the Go vulnerability database and prints the
call path for every reachable vulnerable symbol. Add `-test` to include test code. A
clean run for an advisory that is in the database is a "not reached" verdict with the
evidence already written. Not every GHSA has an entry in the database; when the
advisory is missing from the output and from `govulncheck -scan module`, fall back to
the manual read.

## Bump

```
go get <module>@<patched-version>
go mod tidy
```

Works for transitive modules too: Go takes the highest version any module requires, so
a requirement in the main module's `go.mod` overrides a parent's lower one. There is no
range to edit.

## Outputs that look like failures

- `go get` raising the `go` directive or adding a `toolchain` line: the patched version
  needs a newer Go. A toolchain bump is the user's decision: report it, do not force it.
- `go mod tidy` adding `// indirect` to the new requirement: expected, the main module
  now requires a module it does not import.

## Default verification

```
go build ./...
go vet ./...
go test ./...
```
