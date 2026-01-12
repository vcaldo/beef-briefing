import { useState } from 'react'
import './Avatar.css'

interface AvatarProps {
  photoUrl?: string | null
  firstName: string
  lastName?: string | null
  size?: 'small' | 'medium' | 'large'
  className?: string
}

function getInitials(firstName: string, lastName?: string | null): string {
  const first = firstName.charAt(0).toUpperCase()
  const last = lastName ? lastName.charAt(0).toUpperCase() : ''
  return first + last || '?'
}

export function Avatar({
  photoUrl,
  firstName,
  lastName,
  size = 'medium',
  className = '',
}: AvatarProps) {
  const [imageError, setImageError] = useState(false)
  const showImage = photoUrl && !imageError

  const sizeClass = `avatar-${size}`

  return (
    <div className={`avatar ${sizeClass} ${className}`}>
      {showImage ? (
        <img
          src={photoUrl}
          alt={firstName}
          onError={() => setImageError(true)}
          className="avatar-image"
        />
      ) : (
        <span className="avatar-initials">{getInitials(firstName, lastName)}</span>
      )}
    </div>
  )
}
