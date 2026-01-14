#!/bin/bash
# Ralph - Automated iterative development using Claude
# Reads tasks from a TODO file, executes work, tracks progress, and commits changes

set -e

# =============================================================================
# CONFIGURATION
# =============================================================================
DEFAULT_TODO_FILE="TODO.md"
DEFAULT_PROGRESS_FILE="progress.txt"
DEFAULT_METRICS_FILE="ralph_metrics.jsonl"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Metrics tracking variables
METRICS_LOG="$DEFAULT_METRICS_FILE"
TOTAL_DURATION=0
TOTAL_INPUT_TOKENS=0
TOTAL_OUTPUT_TOKENS=0
TOTAL_FILES_CHANGED=0
INTERACTION_COUNT=0

# Per-iteration metrics
ITERATION_DURATION=0
ITERATION_START=0
ITERATION_MODEL="unknown"
ITERATION_STOP_REASON="unknown"
ITERATION_INPUT_TOKENS=0
ITERATION_OUTPUT_TOKENS=0
ITERATION_CACHE_CREATE_TOKENS=0
ITERATION_CACHE_READ_TOKENS=0
ITERATION_TOTAL_TOKENS=0
ITERATION_FILES_CHANGED=0
ITERATION_SUCCESS="true"
CLAUDE_EXIT_CODE=0

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
# PRICING FUNCTIONS
# =============================================================================

# Get pricing for a Claude model
# Returns: "input_price output_price" (per million tokens)
get_model_pricing() {
    local model="$1"

    # Determine model tier from model name
    if [[ "$model" =~ haiku ]]; then
        echo "0.80 4.00"  # Haiku: $0.80/MTok input, $4.00/MTok output
    elif [[ "$model" =~ sonnet ]]; then
        echo "3.00 12.00"  # Sonnet: $3.00/MTok input, $12.00/MTok output
    elif [[ "$model" =~ opus ]]; then
        echo "15.00 45.00"  # Opus: $15.00/MTok input, $45.00/MTok output
    else
        # Default to Haiku if model tier can't be determined
        echo "0.80 4.00"
    fi
}

# Get the primary model used in this session from metrics log
get_session_model() {
    if [[ ! -f "$METRICS_LOG" ]]; then
        echo "unknown"
        return
    fi

    # Get the model from the first metric entry
    jq -r '.model // "unknown"' "$METRICS_LOG" 2>/dev/null | head -1
}

# =============================================================================
# METRICS FUNCTIONS
# =============================================================================

extract_iteration_metrics() {
    local json="$1"
    local start_time="$2"

    # Calculate duration
    ITERATION_DURATION=$((SECONDS - start_time))

    # Validate JSON output
    if ! echo "$json" | jq empty 2>/dev/null; then
        log_warn "Invalid JSON response, using default metrics"
        ITERATION_MODEL="unknown"
        ITERATION_STOP_REASON="parse_error"
        ITERATION_INPUT_TOKENS=0
        ITERATION_OUTPUT_TOKENS=0
        ITERATION_CACHE_CREATE_TOKENS=0
        ITERATION_CACHE_READ_TOKENS=0
        ITERATION_TOTAL_TOKENS=0
        ITERATION_FILES_CHANGED=0
        ITERATION_SUCCESS="false"
        return 1
    fi

    # Extract metrics from JSON with defaults
    ITERATION_MODEL=$(echo "$json" | jq -r '.modelUsage | keys[0] // "unknown"')
    ITERATION_STOP_REASON=$(echo "$json" | jq -r '.type // "unknown"')
    ITERATION_INPUT_TOKENS=$(echo "$json" | jq -r '.usage.input_tokens // 0')
    ITERATION_OUTPUT_TOKENS=$(echo "$json" | jq -r '.usage.output_tokens // 0')
    ITERATION_CACHE_CREATE_TOKENS=$(echo "$json" | jq -r '.usage.cache_creation_input_tokens // 0')
    ITERATION_CACHE_READ_TOKENS=$(echo "$json" | jq -r '.usage.cache_read_input_tokens // 0')
    ITERATION_TOTAL_TOKENS=$((ITERATION_INPUT_TOKENS + ITERATION_OUTPUT_TOKENS))

    # Calculate files changed (count modified files in git)
    ITERATION_FILES_CHANGED=$(git diff --name-only HEAD 2>/dev/null | wc -l)

    # Determine success status
    ITERATION_SUCCESS="true"
    if [[ $CLAUDE_EXIT_CODE -ne 0 ]]; then
        ITERATION_SUCCESS="false"
    fi

    # Append to metrics log (JSONL format)
    append_metrics_log

    # Update running totals
    TOTAL_DURATION=$((TOTAL_DURATION + ITERATION_DURATION))
    TOTAL_INPUT_TOKENS=$((TOTAL_INPUT_TOKENS + ITERATION_INPUT_TOKENS))
    TOTAL_OUTPUT_TOKENS=$((TOTAL_OUTPUT_TOKENS + ITERATION_OUTPUT_TOKENS))
    TOTAL_FILES_CHANGED=$((TOTAL_FILES_CHANGED + ITERATION_FILES_CHANGED))
    INTERACTION_COUNT=$((INTERACTION_COUNT + 1))
}

append_metrics_log() {
    # Get ISO 8601 timestamp
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Build JSON object using jq for proper escaping
    jq -n \
        --arg iteration "$((INTERACTION_COUNT + 1))" \
        --arg timestamp "$timestamp" \
        --arg duration "$ITERATION_DURATION" \
        --arg model "$ITERATION_MODEL" \
        --arg stop_reason "$ITERATION_STOP_REASON" \
        --arg input_tokens "$ITERATION_INPUT_TOKENS" \
        --arg output_tokens "$ITERATION_OUTPUT_TOKENS" \
        --arg cache_create "$ITERATION_CACHE_CREATE_TOKENS" \
        --arg cache_read "$ITERATION_CACHE_READ_TOKENS" \
        --arg total_tokens "$ITERATION_TOTAL_TOKENS" \
        --arg files_changed "$ITERATION_FILES_CHANGED" \
        --arg success "$ITERATION_SUCCESS" \
        --arg exit_code "$CLAUDE_EXIT_CODE" \
        '{
            iteration: ($iteration | tonumber),
            timestamp: $timestamp,
            duration_seconds: ($duration | tonumber),
            model: $model,
            stop_reason: $stop_reason,
            usage: {
                input_tokens: ($input_tokens | tonumber),
                output_tokens: ($output_tokens | tonumber),
                cache_creation_tokens: ($cache_create | tonumber),
                cache_read_tokens: ($cache_read | tonumber),
                total_tokens: ($total_tokens | tonumber)
            },
            files_changed: ($files_changed | tonumber),
            success: ($success == "true"),
            exit_code: ($exit_code | tonumber)
        }' >> "$METRICS_LOG" || log_warn "Failed to write metrics to log file"
}

print_metrics_summary() {
    # Format duration (convert seconds to minutes:seconds)
    local duration_min=$((ITERATION_DURATION / 60))
    local duration_sec=$((ITERATION_DURATION % 60))
    local duration_str=$(printf "%dm %02ds" "$duration_min" "$duration_sec")

    # Calculate cache hit rate
    local cache_hit_rate="N/A"
    if [[ $ITERATION_INPUT_TOKENS -gt 0 ]]; then
        cache_hit_rate=$(( (ITERATION_CACHE_READ_TOKENS * 100) / ITERATION_INPUT_TOKENS ))
        cache_hit_rate="${cache_hit_rate}%"
    fi

    # Status icon
    local status_icon="✓"
    if [[ "$ITERATION_SUCCESS" == "false" ]]; then
        status_icon="✗"
    fi

    # Print simple list format (no box borders)
    echo "--- Iteration Metrics ---"
    echo "Duration: $duration_str"
    echo "Model: $ITERATION_MODEL"
    echo "Status: $ITERATION_STOP_REASON"
    echo "Input tokens: $ITERATION_INPUT_TOKENS"
    echo "Output tokens: $ITERATION_OUTPUT_TOKENS"
    echo "Total tokens: $ITERATION_TOTAL_TOKENS"
    echo "Cache created: $ITERATION_CACHE_CREATE_TOKENS tokens"
    echo "Cache read: $ITERATION_CACHE_READ_TOKENS tokens ($cache_hit_rate hit rate)"
    echo "Files changed: $ITERATION_FILES_CHANGED"
    echo "Success: $status_icon"
}

print_final_summary() {
    echo ""
    echo "=================================================="
    echo "                  FINAL SUMMARY"
    echo "=================================================="
    echo ""

    # Calculate success rate
    local success_count=$(jq -s '[.[] | select(.success == true)] | length' "$METRICS_LOG" 2>/dev/null || echo "0")
    local success_rate=0
    if [[ $INTERACTION_COUNT -gt 0 ]]; then
        success_rate=$(( (success_count * 100) / INTERACTION_COUNT ))
    fi

    # Calculate average duration
    local avg_duration=0
    if [[ $INTERACTION_COUNT -gt 0 ]]; then
        avg_duration=$((TOTAL_DURATION / INTERACTION_COUNT))
    fi
    local avg_min=$((avg_duration / 60))
    local avg_sec=$((avg_duration % 60))

    # Calculate min/max duration from JSONL
    local min_duration=$(jq -s 'map(.duration_seconds) | min' "$METRICS_LOG" 2>/dev/null || echo "0")
    local max_duration=$(jq -s 'map(.duration_seconds) | max' "$METRICS_LOG" 2>/dev/null || echo "0")

    # Calculate average tokens
    local avg_tokens=0
    if [[ $INTERACTION_COUNT -gt 0 ]]; then
        avg_tokens=$(( (TOTAL_INPUT_TOKENS + TOTAL_OUTPUT_TOKENS) / INTERACTION_COUNT ))
    fi

    # Calculate total cache tokens
    local total_cache_create=$(jq -s 'map(.usage.cache_creation_tokens) | add' "$METRICS_LOG" 2>/dev/null || echo "0")
    local total_cache_read=$(jq -s 'map(.usage.cache_read_tokens) | add' "$METRICS_LOG" 2>/dev/null || echo "0")

    # Calculate overall cache hit rate
    local cache_hit_rate="N/A"
    if [[ $TOTAL_INPUT_TOKENS -gt 0 ]]; then
        cache_hit_rate=$(( (total_cache_read * 100) / TOTAL_INPUT_TOKENS ))
        cache_hit_rate="${cache_hit_rate}%"
    fi

    # Format total duration
    local total_min=$((TOTAL_DURATION / 60))
    local total_sec=$((TOTAL_DURATION % 60))

    # Get actual model used and its pricing
    local session_model=$(get_session_model)
    local pricing=$(get_model_pricing "$session_model")
    read -r input_price output_price <<< "$pricing"

    # Cost estimate based on actual model used
    local input_cost=$(awk "BEGIN {printf \"%.3f\", ($TOTAL_INPUT_TOKENS / 1000000) * $input_price}")
    local output_cost=$(awk "BEGIN {printf \"%.3f\", ($TOTAL_OUTPUT_TOKENS / 1000000) * $output_price}")
    local total_cost=$(awk "BEGIN {printf \"%.3f\", $input_cost + $output_cost}")

    echo "Iterations:"
    echo "  Completed:       $INTERACTION_COUNT / $ITERATIONS"
    echo "  Success Rate:    ${success_rate}%"
    echo ""
    echo "Duration:"
    echo "  Total:           ${total_min}m ${total_sec}s"
    echo "  Average:         ${avg_min}m $(printf "%02d" $avg_sec)s per iteration"
    echo "  Min:             ${min_duration}s"
    echo "  Max:             ${max_duration}s"
    echo ""
    echo "Token Usage:"
    printf "  Total Input:     %d tokens\n" "$TOTAL_INPUT_TOKENS"
    printf "  Total Output:    %d tokens\n" "$TOTAL_OUTPUT_TOKENS"
    printf "  Total:           %d tokens\n" "$((TOTAL_INPUT_TOKENS + TOTAL_OUTPUT_TOKENS))"
    printf "  Average:         %d tokens per iteration\n" "$avg_tokens"
    echo ""
    echo "Cache Performance:"
    printf "  Total Created:   %d tokens\n" "$total_cache_create"
    printf "  Total Read:      %d tokens\n" "$total_cache_read"
    echo "  Overall Hit Rate: $cache_hit_rate"
    echo ""
    echo "Files Changed:"
    echo "  Total:           $TOTAL_FILES_CHANGED files"
    if [[ $INTERACTION_COUNT -gt 0 ]]; then
        echo "  Average:         $((TOTAL_FILES_CHANGED / INTERACTION_COUNT)) files per iteration"
    fi
    echo ""
    echo "Cost Estimate ($session_model):"
    echo "  Input tokens:    \$$input_cost"
    echo "  Output tokens:   \$$output_cost"
    echo "  Total:           \$$total_cost"
    echo ""
    log_info "Metrics log saved to: $METRICS_LOG"
    echo ""
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
# MAIN LOOP
# =============================================================================

for ((i=1; i<=ITERATIONS; i++)); do
    echo "=================================================="
    echo "Iteration $i"
    echo "=================================================="
    echo ""

    # Capture start time for metrics
    ITERATION_START=$SECONDS

    # Run Claude with permission to make edits
    # Use @file syntax to reference files that Claude can read and edit
    # Use --output-format json to capture metrics
    claude_json=$(claude --output-format json --permission-mode acceptEdits -p "Find the highest-priority task from the TODO file and work only on that task.

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

IMPORTANT: Only work on a SINGLE task per iteration.

If, while working on the task, you determine ALL tasks are complete, output exactly this:
<promise>COMPLETE</promise>") 2>&1 || CLAUDE_EXIT_CODE=$?

    # Extract text content from JSON for completion check and display
    result=$(echo "$claude_json" | jq -r '.result // ""')

    # Extract and log metrics
    extract_iteration_metrics "$claude_json" "$ITERATION_START"

    # Display interaction result
    echo "$result"
    echo ""

    # Display metrics summary
    print_metrics_summary
    echo ""

    # Check for completion signal
    if [[ "$result" == *"<promise>COMPLETE</promise>"* ]]; then
        echo "=================================================="
        log_success "All tasks complete, exiting"
        echo "=================================================="

        # Print final summary
        print_final_summary

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

# Print final summary
print_final_summary

log_info "Progress file: $PROGRESS_FILE"
log_info "To continue, run: $0 $ITERATIONS $TODO_FILE $PROGRESS_FILE"
