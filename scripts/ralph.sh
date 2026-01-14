#!/bin/bash
# Ralph - Automated iterative development using Claude
# Reads tasks from a TODO file, executes work, tracks progress, and commits changes

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
DEFAULT_TODO_FILE="TODO.md"
DEFAULT_PROGRESS_FILE="progress.txt"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# =============================================================================
# FUNCTIONS
# =============================================================================

log_error() {
    echo -e "${RED}✗${NC}  $1" >&2
}

log_success() {
    echo -e "${GREEN}✓${NC}  $1"
}

log_info() {
    echo -e "${BLUE}ℹ${NC}  $1"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC}  $1"
}

# =============================================================================
# ARGUMENT PARSING & VALIDATION
# =============================================================================

if [ -z "$1" ]; then
    log_error "Usage: $0 <iterations> [todo-file] [progress-file]"
    echo ""
    echo "Arguments:"
    echo "  iterations      Maximum number of iterations to run"
    echo "  todo-file       Path to TODO file (default: TODO.md)"
    echo "  progress-file   Path to progress file (default: progress.txt)"
    echo ""
    echo "Examples:"
    echo "  $0 10                          # Run 10 iterations on TODO.md"
    echo "  $0 5 docs/tasks.md             # Custom TODO file"
    echo "  $0 10 docs/tasks.md work.txt   # Custom TODO and progress files"
    exit 1
fi

ITERATIONS=$1
TODO_FILE=${2:-$DEFAULT_TODO_FILE}
PROGRESS_FILE=${3:-$DEFAULT_PROGRESS_FILE}

# Validate iterations is a number
if ! [[ "$ITERATIONS" =~ ^[0-9]+$ ]]; then
    log_error "Iterations must be a positive integer, got: $ITERATIONS"
    exit 1
fi

if [ "$ITERATIONS" -eq 0 ]; then
    log_error "Iterations must be greater than 0"
    exit 1
fi

log_info "Ralph Automation Script"
log_info "Iterations: $ITERATIONS"
log_info "TODO file: $TODO_FILE"
log_info "Progress file: $PROGRESS_FILE"
echo ""

# =============================================================================
# FILE SETUP
# =============================================================================

# Warn if TODO file doesn't exist, create empty one
if [ ! -f "$TODO_FILE" ]; then
    log_warn "TODO file not found at $TODO_FILE, creating empty file"
    touch "$TODO_FILE"
fi

# Create progress file if it doesn't exist
if [ ! -f "$PROGRESS_FILE" ]; then
    log_info "Creating progress file at $PROGRESS_FILE"
    touch "$PROGRESS_FILE"
fi

# Verify we can write to progress file
if ! touch "$PROGRESS_FILE" 2>/dev/null; then
    log_error "Cannot write to progress file: $PROGRESS_FILE"
    exit 1
fi

# =============================================================================
# GIT VALIDATION (optional warning)
# =============================================================================

if ! git config user.name > /dev/null 2>&1; then
    log_warn "Git user.name not configured"
    log_warn "Run: git config --global user.name 'Your Name'"
fi

if ! git config user.email > /dev/null 2>&1; then
    log_warn "Git user.email not configured"
    log_warn "Run: git config --global user.email 'your@email.com'"
fi

echo ""

# =============================================================================
# MAIN LOOP
# =============================================================================

for ((i=1; i<=ITERATIONS; i++)); do
    echo "=================================================="
    echo "Iteration $i"
    echo "=================================================="
    echo ""

    # Run Claude with permission to make edits
    # Use @file syntax to reference files that Claude can read and edit
    result=$(claude --permission-mode acceptEdits -p "Find the highest-priority task from the TODO file and work only on that task.

Here are the current TODO items and progress:

@$TODO_FILE

@$PROGRESS_FILE

Guidelines:
1. Pick ONE task from the TODO file that you determine has the highest priority
2. Work ONLY on that task - do not work on multiple tasks
3. Update the TODO file by marking the task as complete (change [ ] to [x]) or updating its status
4. After completing the task, append your progress to the progress file (@$PROGRESS_FILE) with this format:
   - Current date/time
   - Task name
   - What was accomplished
   - Next steps (if any)
5. Make a git commit for the work you completed
   - Use a clear, descriptive commit message
   - The commit will automatically use your configured git user

IMPORTANT: Only work on a SINGLE task per iteration.

If, while working on the task, you determine ALL tasks are complete, output exactly this:
<promise>COMPLETE</promise>")

    echo "$result"
    echo ""

    # Check for completion signal
    if [[ "$result" == *"<promise>COMPLETE</promise>"* ]]; then
        echo "=================================================="
        log_success "All tasks complete, exiting"
        echo "=================================================="
        echo ""

        # Send notification if tt is available
        if command -v tt &> /dev/null; then
            tt notify "Ralph: All tasks complete after $i iterations"
        fi

        exit 0
    fi

    echo ""
done

# If we get here, we ran out of iterations
echo "=================================================="
log_warn "Reached maximum iterations ($ITERATIONS)"
log_info "Tasks may remain incomplete - check $TODO_FILE"
echo "=================================================="
echo ""
log_info "Progress file: $PROGRESS_FILE"
log_info "To continue, run: $0 $ITERATIONS $TODO_FILE $PROGRESS_FILE"
