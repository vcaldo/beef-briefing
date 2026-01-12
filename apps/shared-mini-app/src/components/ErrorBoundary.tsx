import { Component, ReactNode } from 'react'
import { noticeError, addPageAction } from '../monitoring'

interface Props {
  children: ReactNode
  fallback?: ReactNode
  onReset?: () => void
  name?: string
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught error:', error, errorInfo)
    noticeError(error, {
      context: 'error_boundary',
      boundary_name: this.props.name || 'unknown',
      componentStack: errorInfo.componentStack,
    })
    addPageAction('error_boundary_triggered', {
      boundary_name: this.props.name || 'unknown',
      error_message: error.message,
    })
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
    this.props.onReset?.()
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback
      }

      return (
        <div className="error-boundary-fallback">
          <div className="fallback-icon">⚠️</div>
          <div className="fallback-title">Something went wrong</div>
          <div className="fallback-message">
            {this.state.error?.message || 'An unexpected error occurred'}
          </div>
          <button className="btn btn-primary" onClick={this.handleReset}>
            Try Again
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
