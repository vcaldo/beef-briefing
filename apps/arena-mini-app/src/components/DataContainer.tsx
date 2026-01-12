import React from 'react';

interface DataContainerProps {
  loading: boolean;
  error: string | null;
  empty: boolean;
  emptyMessage?: string;
  onRetry?: () => void;
  children: React.ReactNode;
}

/**
 * Reusable container for common loading/error/empty state patterns.
 * Reduces duplication across pages.
 *
 * @example
 * <DataContainer
 *   loading={loading}
 *   error={error}
 *   empty={matches.length === 0}
 *   emptyMessage="No matches found"
 *   onRetry={() => refetch()}
 * >
 *   {children}
 * </DataContainer>
 */
export function DataContainer({
  loading,
  error,
  empty,
  emptyMessage = 'No data available',
  onRetry,
  children,
}: DataContainerProps) {
  if (loading) {
    return (
      <div className="data-container-loading">
        <div className="spinner" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="data-container-error">
        <div className="error-icon">⚠️</div>
        <div className="error-message">{error}</div>
        {onRetry && (
          <button className="btn btn-secondary" onClick={onRetry}>
            Retry
          </button>
        )}
      </div>
    );
  }

  if (empty) {
    return (
      <div className="data-container-empty">
        <div className="empty-icon">📭</div>
        <div className="empty-message">{emptyMessage}</div>
      </div>
    );
  }

  return <>{children}</>;
}
