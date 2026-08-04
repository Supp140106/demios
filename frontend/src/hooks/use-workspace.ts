import { useState, useCallback, useEffect } from "react"
import { SetWorkspace } from "../../wailsjs/go/main/App"

const STORAGE_KEY = "demios-workspace"

export function useWorkspace() {
  const [workspace, setWorkspaceState] = useState<string | null>(() => {
    return localStorage.getItem(STORAGE_KEY)
  })

  // Sync the persisted workspace (from localStorage) to the Go backend on
  // mount. When `wails dev` reloads, the backend restarts with an empty
  // workspace, so we must push the stored value back to keep the agent
  // aware of the current directory.
  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      SetWorkspace(stored).catch(() => {})
    }
  }, [])

  const setWorkspace = useCallback(async (path: string) => {
    try {
      await SetWorkspace(path)
      localStorage.setItem(STORAGE_KEY, path)
      setWorkspaceState(path)
    } catch {
      // Silently fail — user can retry
    }
  }, [])

  const clearWorkspace = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY)
    setWorkspaceState(null)
  }, [])

  return { workspace, setWorkspace, clearWorkspace }
}
