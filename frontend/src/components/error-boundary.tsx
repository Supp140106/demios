import { Component, type ReactNode } from "react"

type Props = { children: ReactNode }
type State = { hasError: boolean; message?: string }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(error: unknown): State {
    return {
      hasError: true,
      message: error instanceof Error ? error.message : String(error),
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex h-dvh flex-col items-center justify-center gap-3 p-6 text-center">
          <p className="text-sm font-medium">Something went wrong</p>
          <p className="max-w-sm text-xs text-muted-foreground">
            {this.state.message}
          </p>
          <button
            type="button"
            className="rounded-md border px-3 py-1.5 text-xs"
            onClick={() => window.location.reload()}
          >
            Reload
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
