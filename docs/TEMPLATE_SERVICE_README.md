# [Service Name]

[One-line description of what the service does.]

## Overview

[2-3 sentences describing the service's purpose, role in the system, and key value proposition.]

## Features

- **[Feature Name]**: [Brief description of what it does]
- **[Feature Name]**: [Brief description of what it does]
- **[Feature Name]**: [Brief description of what it does]

## Quick Start

```bash
# Start with Docker (recommended)
make up-build

# Or run locally
cd apps/[service-name]
[language-specific commands]
```

**Access Points:**
- [Endpoint]: http://localhost:[port]
- Health: http://localhost:[port]/health

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `VAR_NAME` | `default` | What it controls |
| `VAR_NAME` | `default` | What it controls |

## API Reference

_Include this section if the service exposes HTTP endpoints._

### [Category Name]

#### METHOD `/path/to/endpoint`

[Brief description of what this endpoint does.]

**Parameters:**
- `param_name` (required/optional): Description

**Request:**
```bash
curl -X [METHOD] http://localhost:[port]/path \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}'
```

**Response:**
```json
{
  "key": "value"
}
```

---

## Architecture

```
apps/[service-name]/
├── cmd/main.go                 # Entry point
├── internal/
│   ├── handlers/               # HTTP handlers
│   ├── services/               # Business logic
│   └── [other]/                # Other packages
└── Dockerfile
```

### Data Flow

_Include ASCII diagram showing how data flows through the service._

```
[Input Source] → [Service] → [Processing] → [Output/Storage]
```

## Troubleshooting

### [Problem description]

[Solution or diagnostic steps]

```bash
# Commands to diagnose or fix
```

### [Another problem]

[Solution or diagnostic steps]

---

## Template Usage Guide

_Delete this section when using the template._

### Required Sections

Every service README must include:

1. **Title + One-liner**: Clear service name and purpose
2. **Overview**: 2-3 sentences on what it does
3. **Features**: Bulleted list with bold feature names
4. **Quick Start**: Copy-paste ready commands
5. **Configuration**: Table with Variable | Default | Description
6. **Architecture**: Project structure as file tree
7. **Troubleshooting**: Common problems and solutions

### Optional Sections

Include when applicable:

- **API Reference**: For services with HTTP endpoints
- **Authentication**: If the service has auth mechanisms
- **Database Schema**: For services with data storage
- **Data Flow Diagrams**: For complex processing pipelines
- **Concurrency Patterns**: For services with non-trivial threading
- **Internal Constants**: For configurable but not env-var values

### Formatting Standards

**Tables**: Always use 3-column format for configuration:
```markdown
| Variable | Default | Description |
|----------|---------|-------------|
```

**Code Examples**: Always include:
- Language tag (bash, json, go, etc.)
- Real, working values (not placeholders like `<your-value>`)
- Comments for non-obvious steps

**Feature Lists**: Start with bold name + colon:
```markdown
- **Feature Name**: Description of what it does
```

**Headers**: Use consistent hierarchy:
- h1 (#): Title only
- h2 (##): Major sections
- h3 (###): Subsections (API categories, etc.)
- h4 (####): Specific items (individual endpoints)

**Links**: Use relative paths for internal docs:
```markdown
See [api-service README](../api-service/README.md)
```

### What NOT to Include

- Time estimates ("takes about 5 minutes")
- Version numbers that will go stale
- Redundant info already in CLAUDE.md or other docs
- Placeholder values like `<your-token-here>`
