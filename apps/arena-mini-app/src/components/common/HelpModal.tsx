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
    <div className="rpg-modal-backdrop" onClick={onClose}>
      <div className="rpg-modal-wrapper" onClick={(e) => e.stopPropagation()}>
        <RPGPanel variant="outer" className="rpg-modal-outer">
          {/* Close button */}
          <div className="rpg-modal-close-wrapper">
            <GameButton variant="danger" size="sm" shape="square" onClick={onClose}>
              ×
            </GameButton>
          </div>

          {/* Content - wrapped in inner panel */}
          <RPGPanel variant="inner" className="rpg-modal-inner">
            <div className="help-content">
              {/* Language toggle */}
              <div className="help-language-toggle">
                <button
                  className={`lang-btn ${language === 'pt' ? 'active' : ''}`}
                  onClick={() => setLanguage('pt')}
                >
                  <img src="https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f1f5-1f1f9.png" alt="PT" className="flag-emoji" />
                </button>
                <button
                  className={`lang-btn ${language === 'en' ? 'active' : ''}`}
                  onClick={() => setLanguage('en')}
                >
                  <img src="https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f1ef-1f1f2.png" alt="JM" className="flag-emoji" />
                </button>
              </div>
              {language === 'en' ? <HelpContentEN /> : <HelpContentPT />}
            </div>
          </RPGPanel>
        </RPGPanel>
      </div>
    </div>
  )
}
