import type { AppPage } from '../types';
import './Navigation.css';

interface NavigationProps {
  currentPage: AppPage;
  onNavigate: (page: AppPage) => void;
}

export function Navigation({ currentPage, onNavigate }: NavigationProps) {
  const tabs: { page: AppPage; icon: string; label: string }[] = [
    { page: 'lobby', icon: '🏠', label: 'Home' },
    { page: 'leaderboard', icon: '🏆', label: 'Ranks' },
    { page: 'history', icon: '📜', label: 'History' },
  ];

  return (
    <nav className="navigation">
      {tabs.map(({ page, icon, label }) => (
        <button
          key={page}
          className={`nav-tab ${currentPage === page ? 'active' : ''}`}
          onClick={() => onNavigate(page)}
        >
          <span className="nav-tab-icon">{icon}</span>
          <span className="nav-tab-label">{label}</span>
        </button>
      ))}
    </nav>
  );
}
