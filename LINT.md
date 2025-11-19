# Linting

This repository uses a comprehensive linting setup for TypeScript, Go, Python, and Shell scripts.
The goal is to ensure consistent code quality across all supported languages and to provide a
portable linting experience that works on most machines.

## Usage

To run all linters:

```bash
npm run lint
```

To run individual linters:

```bash
npm run lint:ts   # TypeScript
npm run lint:go   # Go (runs both fmt and vet)
npm run lint:py   # Python
npm run lint:sh   # Shell
```

## Configuration & Mechanism

### TypeScript (`lint:ts`)

- **Tool:** `eslint`
- **Config:** `.eslintrc.js`
- **Mechanism:** Runs via `npm`. Uses `ESLINT_USE_FLAT_CONFIG=false` to ensure compatibility with
  the current configuration format.

### Go (`lint:go`)

- **Tools:** `gofmt`, `go vet`
- **Mechanism:** Runs hermetically via Bazel.
  - `npm run lint:go:fmt` executes `bazel run //:gofmt`
  - `npm run lint:go:vet` executes `bazel run //:go -- vet`

### Python (`lint:py`)

- **Tool:** `pylint`
- **Config:** `.pylintrc`
- **Dependencies:** `requirements.txt`
- **Mechanism:** Runs hermetically via Bazel. `rules_python` in `WORKSPACE` uses `requirements.txt`
  to fetch exactly the right version of `pylint` and its dependencies, ensuring that every
  developer runs the exact same linter regardless of what is installed on their host machine.
- **Note:** `requirements.txt` is functionally required by the build system and cannot be removed
  without breaking the hermetic Python linting.

### Shell (`lint:sh`)

- **Tool:** `shellcheck`
- **Mechanism:** Runs via `npm`. `shellcheck` is installed as a `devDependency` in `package.json`,
  ensuring a consistent version is used across environments.
