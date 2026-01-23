#!/bin/bash
# Normalize audio files using EBU R128 loudness normalization
# Usage: ./scripts/normalize-audio.sh [--dry-run] [--lufs=-16] [DIR]
#
# Applies loudness normalization to all .ogg files in the target directory,
# backing up originals to assets/backup/ with timestamp.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# =============================================================================
# CONFIGURATION
# =============================================================================

# Default target loudness (-16 LUFS is standard for games/streaming)
TARGET_LUFS=-16

# True peak limit (headroom to prevent clipping)
TRUE_PEAK=-1.5

# Loudness range target
LRA=11

# Ogg Vorbis quality (4 = ~128kbps, good balance of size/quality)
VORBIS_QUALITY=4

# Defaults
DRY_RUN=false
SOUNDS_DIR="$PROJECT_ROOT/apps/arena-mini-app/assets/sounds"
BACKUP_DIR="$PROJECT_ROOT/assets/backup"

# =============================================================================
# FUNCTIONS
# =============================================================================

show_usage() {
    echo "Usage: $0 [OPTIONS] [DIR]"
    echo ""
    echo "Normalize audio files using EBU R128 loudness normalization."
    echo "Backs up originals to assets/backup/ with timestamp."
    echo ""
    echo "Options:"
    echo "  --dry-run       Preview what would be processed without making changes"
    echo "  --lufs=VALUE    Target loudness in LUFS (default: -16)"
    echo "  --help, -h      Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Normalize all arena sounds"
    echo "  $0 --dry-run          # Preview without changes"
    echo "  $0 --lufs=-14         # Use -14 LUFS target (louder)"
    echo "  $0 /path/to/sounds    # Normalize custom directory"
}

# =============================================================================
# ARGUMENT PARSING
# =============================================================================

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --lufs=*)
            TARGET_LUFS="${1#*=}"
            shift
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
        -*)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
        *)
            SOUNDS_DIR="$1"
            shift
            ;;
    esac
done

# =============================================================================
# VALIDATION
# =============================================================================

# Check ffmpeg is installed
if ! command -v ffmpeg &> /dev/null; then
    log_error "ffmpeg is not installed"
    log_info "Install with: sudo apt install ffmpeg (Ubuntu) or brew install ffmpeg (macOS)"
    exit 1
fi

# Check source directory exists
if [[ ! -d "$SOUNDS_DIR" ]]; then
    log_error "Source directory not found: $SOUNDS_DIR"
    exit 1
fi

# =============================================================================
# MAIN
# =============================================================================

# Create timestamped backup directory name
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_PATH="$BACKUP_DIR/$TIMESTAMP"

log_step "Audio Normalization (EBU R128)"
log_info "Source: $SOUNDS_DIR"
log_info "Backup: $BACKUP_PATH"
log_info "Target: ${TARGET_LUFS} LUFS, TP: ${TRUE_PEAK} dBTP"
echo ""

# Find all .ogg files recursively
mapfile -t FILES < <(find "$SOUNDS_DIR" -name "*.ogg" -type f | sort)
TOTAL=${#FILES[@]}

if [[ $TOTAL -eq 0 ]]; then
    log_warn "No .ogg files found in $SOUNDS_DIR"
    exit 0
fi

log_info "Found $TOTAL file(s) to process"

if [[ "$DRY_RUN" == "true" ]]; then
    log_warn "DRY RUN - no changes will be made"
fi

echo ""

# Create backup directory (unless dry run)
if [[ "$DRY_RUN" == "false" ]]; then
    mkdir -p "$BACKUP_PATH"
fi

PROCESSED=0
FAILED=0

for file in "${FILES[@]}"; do
    filename=$(basename "$file")
    relative_path="${file#$SOUNDS_DIR/}"

    echo -n "  Processing $relative_path... "

    if [[ "$DRY_RUN" == "true" ]]; then
        echo -e "${YELLOW}skipped (dry-run)${NC}"
        continue
    fi

    # Create backup subdirectory structure (preserve folder hierarchy)
    backup_subdir="$BACKUP_PATH/$(dirname "$relative_path")"
    mkdir -p "$backup_subdir"

    # Backup original
    cp "$file" "$backup_subdir/$filename"

    # Process with ffmpeg
    temp_file="${file}.tmp.ogg"
    if ffmpeg -y -i "$file" \
        -af "loudnorm=I=${TARGET_LUFS}:TP=${TRUE_PEAK}:LRA=${LRA}" \
        -c:a libvorbis -q:a "$VORBIS_QUALITY" \
        "$temp_file" 2>/dev/null; then
        mv "$temp_file" "$file"
        echo -e "${GREEN}done${NC}"
        ((++PROCESSED))
    else
        rm -f "$temp_file"
        # Restore from backup on failure
        cp "$backup_subdir/$filename" "$file"
        echo -e "${RED}failed${NC}"
        ((++FAILED))
    fi
done

echo ""
log_info "Processed: $PROCESSED, Failed: $FAILED"

if [[ "$DRY_RUN" == "false" && $PROCESSED -gt 0 ]]; then
    log_success "Originals backed up to: $BACKUP_PATH"
    echo ""
    log_info "To restore originals:"
    echo "  cp -r \"$BACKUP_PATH/\"* \"$SOUNDS_DIR/\""
fi

if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
