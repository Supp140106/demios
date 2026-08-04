import { useState, useCallback, useRef } from "react"
import { ListWorkspaceFiles } from "../../wailsjs/go/main/App"

export type FileEntry = {
  path: string
  name: string
  size: number
  is_dir: boolean
}

export function useFileSearch() {
  const [results, setResults] = useState<FileEntry[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const search = useCallback((query: string) => {
    if (timerRef.current) clearTimeout(timerRef.current)

    setIsSearching(true)

    timerRef.current = setTimeout(async () => {
      try {
        const pattern = query.trim() ? `**/*${query}*` : ""
        const files = await ListWorkspaceFiles(pattern)
        setResults(files ?? [])
      } catch {
        setResults([])
      } finally {
        setIsSearching(false)
      }
    }, 150)
  }, [])

  const reset = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current)
    setResults([])
    setIsSearching(false)
  }, [])

  return { results, isSearching, search, reset }
}
