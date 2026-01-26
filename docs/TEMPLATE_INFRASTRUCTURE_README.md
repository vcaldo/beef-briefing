# [Infrastructure Component]

[One-line description of what this infrastructure component manages.]

## Overview

[2-3 sentences describing the component's purpose and how it fits into the deployment architecture.]

## Directory Structure

```
infrastructure/[component]/
├── [file]                  # Description
├── [file]                  # Description
└── [directory]/
    └── [file]              # Description
```

## Quick Start

### Development

```bash
# Step 1: Description
[command]

# Step 2: Description
[command]
```

### Production

```bash
# Step 1: Description
[command]

# Step 2: Description
[command]
```

## Environment Files

| File | Purpose |
|------|---------|
| `.env.dev` | Development environment configuration |
| `.env.prod` | Production environment configuration |
| `.env.*.example` | Template files with documentation |

## Required Variables

### Development

| Variable | Default | Description |
|----------|---------|-------------|
| `VAR_NAME` | `default` | What it controls |

### Production

| Variable | Default | Description |
|----------|---------|-------------|
| `VAR_NAME` | `default` | What it controls |

## Services

| Service | Port | Language | Description |
|---------|------|----------|-------------|
| [service-name] | [port] | [Go/Python/etc] | [Brief description] |

## Commands Reference

| Command | Description |
|---------|-------------|
| `make [command]` | [What it does] |
| `make [command]` | [What it does] |

## Troubleshooting

### [Problem description]

**Symptoms**: [What the user observes]

**Solution**:
```bash
# Commands to diagnose or fix
```

### [Another problem]

**Symptoms**: [What the user observes]

**Solution**: [Steps to resolve]

## Related Documentation

- [Related doc 1](path/to/doc.md)
- [Related doc 2](path/to/doc.md)

---

## Template Usage Guide

_Delete this section when using the template._

### Required Sections for Infrastructure Docs

1. **Title + One-liner**: Component name and purpose
2. **Overview**: How it fits in the architecture
3. **Directory Structure**: File tree with descriptions
4. **Quick Start**: Separate dev and prod instructions
5. **Environment Files**: Table of env file purposes
6. **Required Variables**: Tables split by environment
7. **Services**: Table with Port, Language, Description
8. **Commands Reference**: Makefile commands table
9. **Troubleshooting**: Problem → Symptoms → Solution format
10. **Related Documentation**: Links to connected docs

### Environment Variable Documentation

Always include:
- Variable name in backticks
- Default value (or "required" if none)
- Clear description of what it controls
- Any validation rules or accepted values

### Command Documentation

For Makefile commands:
- Show the actual command name
- Brief description of what it does
- Note any required arguments or flags

### Troubleshooting Format

Use this structure:
```markdown
### [Problem title - short]

**Symptoms**: What the user sees or experiences

**Cause**: Why this happens (optional, if not obvious)

**Solution**:
[Steps or commands to fix]
```
