# Ralph Script - Quick Reference

Automated iterative development using Claude. Ralph reads tasks from a TODO file, executes work, tracks progress, and makes git commits.

## Quick Start

```bash
# 1. Create a TODO.md file with tasks
cp TODO.md.example TODO.md
# Edit TODO.md with your tasks

# 2. Run Ralph
./scripts/ralph.sh 10

# 3. Watch progress
tail -f progress.txt
```

## Usage

### Basic Command
```bash
./scripts/ralph.sh <iterations> [todo-file] [progress-file]
```

### Arguments
- **iterations** (required): Maximum number of iterations (1+)
- **todo-file** (optional): Path to TODO file (default: `TODO.md`)
- **progress-file** (optional): Path to progress file (default: `progress.txt`)

### Examples

**Run 10 iterations on default TODO.md**
```bash
./scripts/ralph.sh 10
```

**Run with custom TODO file**
```bash
./scripts/ralph.sh 5 docs/arena-tasks.md
```

**Run with both custom files**
```bash
./scripts/ralph.sh 20 docs/tasks.md logs/progress.txt
```

**Run via make (if target added)**
```bash
make ralph ITERATIONS=10
```

## How Ralph Works

### Per Iteration
1. **Displays iteration number** and separator
2. **Calls Claude** with permission to make edits
3. **Passes** TODO file and progress file for Claude to read
4. **Claude**:
   - Identifies highest-priority incomplete task
   - Works on ONLY that single task
   - Updates TODO.md (marks task complete with [x])
   - Appends summary to progress file
   - Makes a git commit (using your git identity)
5. **Script checks** for completion signal
6. **If complete**: Exits immediately
7. **If incomplete**: Continues to next iteration (up to max)

### Exit Conditions
- **Early exit**: When Claude outputs `<promise>COMPLETE</promise>`
- **Normal exit**: When all iterations complete

## File Formats

### TODO.md Format
Standard markdown with checkbox tasks:

```markdown
# Tasks

## High Priority
- [ ] Feature A
- [ ] Bug fix B

## Medium Priority
- [ ] Feature C

## Completed
- [x] Feature X
```

**Rules**:
- Use `- [ ]` for incomplete tasks
- Use `- [x]` for completed tasks
- Ralph reads unchecked items as work to do
- Organize by priority (high → medium → low)

### progress.txt Format
Freeform log file. Ralph appends entries like:

```
2026-01-14 10:30:00 - Feature A
  Completed: Setup database schema and migrations
  Next: Add API endpoints

2026-01-14 11:15:00 - Bug fix B
  Completed: Fixed race condition in login flow
  Tests: All passing
```

**Format is flexible** - Claude appends what makes sense for each task.

## Requirements

### Git Configuration
Ralph commits use your git identity. Verify it's set:

```bash
# Check current config
git config user.name
git config user.email

# Set global config (if needed)
git config --global user.name "Your Name"
git config --global user.email "your@email.com"

# Set for this repo only
git config user.name "Your Name"
git config user.email "your@email.com"
```

### Claude CLI
Requires `claude` command to be installed and in PATH:

```bash
# Verify Claude is available
which claude
claude --version
```

## Workflow Examples

### Example 1: Arena Mini-App Development
```bash
# Create tasks
cat > TODO.md << 'EOF'
# Arena Mini-App Tasks

## High Priority
- [ ] Implement match lobby screen
- [ ] Implement shop phase screen
- [ ] Implement battle display

## Medium Priority
- [ ] Add animations
- [ ] Polish UI

## Low Priority
- [ ] Accessibility improvements
- [ ] Documentation
EOF

# Run ralph
./scripts/ralph.sh 20

# Check progress
cat progress.txt

# Review commits
git log --oneline -20
```

### Example 2: Bug Fix Sprint
```bash
# Create bug list
cat > bugs.md << 'EOF'
# Critical Bugs

## High Priority
- [ ] Fix API timeout issue
- [ ] Fix race condition in shop
- [ ] Fix HP bar animation

## Medium Priority
- [ ] Fix typo in leaderboard
EOF

# Run ralph on bug list
./scripts/ralph.sh 10 bugs.md bug_progress.txt

# Clean up completed bugs
grep -n "\\[x\\]" bugs.md
```

### Example 3: Feature Development
```bash
# Feature tasks
cat > features.md << 'EOF'
# New Features

## High Priority
- [ ] Dark mode support
- [ ] Offline support
- [ ] Export to CSV
EOF

# Run with monitoring
./scripts/ralph.sh 15 features.md feature_progress.txt &
watch -n 5 "tail -10 feature_progress.txt"
```

## Monitoring & Logs

### Watch Progress in Real-time
```bash
# Terminal 1: Run Ralph
./scripts/ralph.sh 20

# Terminal 2: Monitor progress
tail -f progress.txt

# Terminal 3: Watch git commits
watch -n 2 "git log --oneline -5"
```

### Check Status Between Runs
```bash
# See what's been completed
grep "\\[x\\]" TODO.md

# See what's pending
grep "\\[ \\]" TODO.md

# Check last progress entries
tail -20 progress.txt

# Check recent commits
git log --oneline -10
```

## Tips & Best Practices

### ✅ Do This
- Keep task descriptions clear and specific
- Organize by priority (high → medium → low)
- Use simple, actionable language
- Add context comments if tasks are complex
- Check git log to verify commits are using your identity
- Save and review progress.txt frequently

### ❌ Avoid This
- Don't give Ralph conflicting or vague tasks
- Don't include too many tasks (start with 5-10)
- Don't modify TODO.md while Ralph is running
- Don't modify progress.txt directly (let Ralph append)
- Don't set iterations too high without monitoring

### 🎯 Optimization Tips
- Start with fewer iterations (5) to test workflow
- Use clear, specific task names
- Group related work in single tasks
- Monitor first run to check quality
- Increase iterations once you see good results

## Troubleshooting

### Script exits immediately
**Check**: Did you set git config?
```bash
git config user.name
git config user.email
```

### No progress appearing
**Check**: Can Ralph write to files?
```bash
touch progress.txt  # Verify writable
ls -l TODO.md progress.txt
```

### Ralph keeps working on same task
**Check**: Is task properly formatted in TODO.md?
- Use `- [ ]` (with space) for incomplete
- Use `- [x]` for completed
- Check for typos in task names

### Git commits not using correct author
**Check**: Is git configured for this repo?
```bash
git config --list | grep user
# If not set, use: git config user.name "Your Name"
```

### Claude not found error
**Check**: Is Claude CLI installed?
```bash
which claude
claude --version
```

## Integration with Make

### Add to Makefile
```makefile
# Ralph automation
.PHONY: ralph
ralph:
	@bash scripts/ralph.sh $(ITERATIONS) $(TODO_FILE) $(PROGRESS_FILE)
```

### Usage via make
```bash
make ralph ITERATIONS=10
make ralph ITERATIONS=10 TODO_FILE=docs/tasks.md
```

## Advanced Usage

### Parallel tasks (different terminals)
```bash
# Terminal 1: Bug fixes
./scripts/ralph.sh 10 bugs.md bug_progress.txt

# Terminal 2: Features (don't overlap files)
./scripts/ralph.sh 10 features.md feature_progress.txt
```

### Resume from checkpoint
```bash
# First run
./scripts/ralph.sh 10 TODO.md progress.txt

# Resume (manually update iterations in TODO.md, then run again)
./scripts/ralph.sh 15 TODO.md progress.txt
```

### Custom notification
```bash
# Ralph will use 'tt notify' if available
# Install tt: npm install -g @sanity/notification

# Custom notification setup
function claude() {
  /usr/local/bin/claude "$@"
  notify-send "Claude completed iteration"  # Linux
  # or
  osascript -e 'display notification "Done"'  # macOS
}
```

## Output & Debugging

### Standard output
```
Ralph Automation Script
ℹ  Iterations: 10
ℹ  TODO file: TODO.md
ℹ  Progress file: progress.txt

==================================================
Iteration 1
==================================================

[Claude working...]

[Checking for completion signal...]

==================================================
Iteration 2
==================================================
[...]
```

### Enable bash debug mode
```bash
bash -x ./scripts/ralph.sh 5
```

## Files Reference

| File | Purpose | Created by |
|------|---------|-----------|
| `TODO.md` | Task list | You (required) |
| `progress.txt` | Progress log | Ralph (auto-created) |
| `scripts/ralph.sh` | Main script | Provided |
| `scripts/RALPH.md` | This guide | Provided |
| `TODO.md.example` | Example format | Provided |

## See Also

- [CLAUDE.md](../CLAUDE.md) - Project overview
- [Makefile](../Makefile) - Build targets
- [scripts/](../scripts/) - Other utility scripts

## Contributing

To improve Ralph:
1. Test with different task types
2. Report edge cases or bugs
3. Share workflow tips with team
4. Suggest enhancements (see plan file)

---

**Ralph** - Automate your development workflow with Claude
