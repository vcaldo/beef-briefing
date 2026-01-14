interface CreateMatchButtonProps {
  onCreate: () => void
  disabled?: boolean
  isLoading?: boolean
}

/**
 * Prominent button to create a new match.
 */
export function CreateMatchButton({
  onCreate,
  disabled = false,
  isLoading = false,
}: CreateMatchButtonProps) {
  return (
    <button
      className="btn btn-primary btn-lg create-match-btn"
      onClick={onCreate}
      disabled={disabled || isLoading}
    >
      {isLoading ? (
        <>
          <span className="btn-spinner" />
          Creating...
        </>
      ) : (
        <>
          <span>⚔️</span>
          Create Match
        </>
      )}
    </button>
  )
}
