# bcr

Brief description of what this CLI does.

## Using This Template

This is a GitHub template repository. To create a new CLI project:

### Option 1: GitHub Template (Recommended)

1. Click "Use this template" on GitHub
2. Clone your new repository
3. Run the bootstrap script:

```bash
./scripts/setup.sh -o myuser -r my-cli
```

### Option 2: Manual Clone

```bash
git clone https://github.com/richhaase/go-cli-template.git my-cli
cd my-cli
rm -rf .git
git init
./scripts/setup.sh -o myuser -r my-cli
```

### Bootstrap Options

```bash
./scripts/setup.sh --help

# Interactive mode
./scripts/setup.sh

# Non-interactive
./scripts/setup.sh -o myuser -r my-cli -b mycmd -d "My awesome CLI tool"

# Skip confirmation
./scripts/setup.sh -o myuser -r my-cli -y
```

---

## Installation

### From source

```bash
go install github.com/richhaase/bcr/cmd/bcr@latest
```

### From releases

Download the latest binary from [Releases](https://github.com/richhaase/bcr/releases).

### Using make

```bash
git clone https://github.com/richhaase/bcr.git
cd bcr
make install
```

## Usage

```bash
bcr [command] [flags]
```

### Commands

- `example` - An example command to demonstrate patterns
- `version` - Print version information

### Examples

```bash
# Run the example command
bcr example "Hello"

# With flags
bcr example --name "Developer" --count 3

# Check version
bcr version
```

## Development

This project uses `make` as its command runner.

```bash
# List available targets
make help

# Build the binary
make build

# Run tests
make test

# Run all quality checks (non-mutating: fmt-check, vet, lint, test)
make check

# Scan dependencies for known vulnerabilities
make vuln

# Update all dependencies to latest (run periodically to avoid rot)
make deps-update

# Verify the GoReleaser config locally before tagging a release
make release-snapshot

# Clean build artifacts
make clean
```

### Pre-commit hooks

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install
```

## License

MIT License - see [LICENSE](LICENSE) for details.
