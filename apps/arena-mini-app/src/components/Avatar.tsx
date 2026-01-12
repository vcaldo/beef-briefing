import './Avatar.css';

interface AvatarProps {
  src?: string | null;
  name: string;
  size?: 'sm' | 'md' | 'lg';
}

export function Avatar({ src, name, size = 'md' }: AvatarProps) {
  const sizeClass = `avatar-${size}`;

  if (src) {
    return (
      <img
        className={`avatar ${sizeClass}`}
        src={src}
        alt={name}
        onError={(e) => {
          // On error, hide img and show fallback by adding error class
          e.currentTarget.style.display = 'none';
        }}
      />
    );
  }

  // Fallback: show first letter
  return (
    <div className={`avatar avatar-fallback ${sizeClass}`}>
      {name.charAt(0).toUpperCase()}
    </div>
  );
}
