import { useState, useCallback } from "react"
import { SetWorkspace } from "../../wailsjs/go/main/App"

const STORAGE_KEY = "demios-workspace"

export function useWorkspace() {
  const [workspace, setWorkspaceState] = useState<string | null>(() => {
    return localStorage.getItem(STORAGE_KEY)
  })

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
