import { useState, useEffect } from 'react'
import { RPGPanel } from '../ui/RPGPanel'
import { GameButton } from '../ui/GameButton'
import { HelpContentEN, HelpContentPT } from './HelpContent'
import './HelpModal.css'

interface HelpModalProps {
  isOpen: boolean
  onClose: () => void
}

export function HelpModal({ isOpen, onClose }: HelpModalProps) {
  const [language, setLanguage] = useState<'en' | 'pt'>(() => {
    return (localStorage.getItem('help-language') as 'en' | 'pt') || 'en'
  })

  useEffect(() => {
    localStorage.setItem('help-language', language)
  }, [language])

  if (!isOpen) return null

  return (
    <div className="help-modal-backdrop" onClick={onClose}>
      <div className="help-modal-wrapper" onClick={(e) => e.stopPropagation()}>
        <RPGPanel variant="outer" className="help-modal-outer">
          {/* Close button */}
          <div className="help-close-wrapper">
            <GameButton variant="danger" size="sm" shape="square" onClick={onClose}>
              ×
            </GameButton>
          </div>

          {/* Content */}
          <div className="help-content">
            {/* Language toggle */}
            <div className="help-language-toggle">
              <button
                className={`lang-btn ${language === 'pt' ? 'active' : ''}`}
                onClick={() => setLanguage('pt')}
              >
                🇵🇹
              </button>
              <button
                className={`lang-btn ${language === 'en' ? 'active' : ''}`}
                onClick={() => setLanguage('en')}
              >
                🇯🇲
              </button>
            </div>
            {language === 'en' ? <HelpContentEN /> : <HelpContentPT />}
          </div>
        </RPGPanel>
      </div>
    </div>
  )
}
