import helpIconUrl from '../../../assets/images/icons/pentagon_question.png'
import './HelpButton.css'

interface HelpButtonProps {
  onClick: () => void
}

export function HelpButton({ onClick }: HelpButtonProps) {
  return (
    <button
      className="help-button"
      onClick={onClick}
      aria-label="Help"
      title="Game Help"
    >
      <img src={helpIconUrl} alt="?" className="help-icon" />
    </button>
  )
}
