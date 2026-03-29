import { Component } from 'react'
import type { ReactNode, ErrorInfo } from 'react'

interface Props  { children: ReactNode }
interface State  { error: Error | null }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Uncaught error:', error, info)
  }

  render() {
    if (this.state.error) {
      return (
        <div className="min-h-screen flex items-center justify-center p-8">
          <div className="max-w-lg text-center">
            <h1 className="font-display text-3xl text-text-primary mb-3">Something went wrong</h1>
            <p className="text-text-secondary text-sm mb-6">
              An unexpected error occurred. Reload the page to continue.
            </p>
            <pre className="text-left bg-surface-alt border border-border rounded-lg p-4 text-xs text-red-600 overflow-auto mb-6">
              {this.state.error.message}
            </pre>
            <button
              onClick={() => window.location.reload()}
              className="px-6 py-2.5 bg-brand text-white rounded-lg font-sans font-semibold text-sm hover:bg-brand-dark transition-colors"
            >
              Reload page
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
