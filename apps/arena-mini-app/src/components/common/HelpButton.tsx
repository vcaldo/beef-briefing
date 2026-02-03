import { useImages } from '../../hooks/useImages'
import './HelpButton.css'

interface HelpButtonProps {
  onClick: () => void
}

export function HelpButton({ onClick }: HelpButtonProps) {
  const { getUrlById } = useImages()

  return (
    <button
      className="help-button"
      onClick={onClick}
      aria-label="Help"
      title="Game Help"
    >
      <img src={getUrlById('pentagon_question')} alt="?" className="help-icon" />
    </button>
  )
}
