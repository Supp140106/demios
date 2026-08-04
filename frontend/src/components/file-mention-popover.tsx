import { useEffect, useRef } from "react"
import type { FileEntry } from "@/hooks/use-file-search"
import { FileText, Folder, Search } from "lucide-react"
import { cn } from "@/lib/utils"

type FileMentionPopoverProps = {
  files: FileEntry[]
  isSearching: boolean
  isOpen: boolean
  query: string
  highlightedIndex: number
  onSelect: (file: FileEntry) => void
  position: { top: number; left: number } | null
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)}KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`
}

export function FileMentionPopover({
  files,
  isSearching,
  isOpen,
  query,
  highlightedIndex,
  onSelect,
  position,
}: FileMentionPopoverProps) {
  const listRef = useRef<HTMLDivElement>(null)
  const itemRefs = useRef<(HTMLDivElement | null)[]>([])

  useEffect(() => {
    if (highlightedIndex >= 0 && itemRefs.current[highlightedIndex]) {
      itemRefs.current[highlightedIndex]?.scrollIntoView({ block: "nearest" })
    }
  }, [highlightedIndex])

  if (!isOpen || !position) return null

  return (
    <div
      className="absolute z-50 max-h-72 w-80 overflow-hidden rounded-xl border border-border bg-popover shadow-lg"
      style={{ bottom: position.top, left: position.left }}
    >
      <div className="flex items-center gap-2 border-b border-border px-3 py-2 text-xs text-muted-foreground">
        <Search className="size-3.5" />
        <span>
          {isSearching
            ? "Searching..."
            : files.length === 0
              ? "No files found"
              : `${files.length} file${files.length !== 1 ? "s" : ""}`}
        </span>
        {query && (
          <span className="ml-auto font-mono text-foreground/70">{query}</span>
        )}
      </div>
      <div ref={listRef} className="overflow-y-auto p-1">
        {files.map((file, i) => (
          <div
            key={file.path}
            ref={(el) => {
              itemRefs.current[i] = el
            }}
            onClick={() => onSelect(file)}
            className={cn(
              "flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-sm transition-colors",
              i === highlightedIndex
                ? "bg-accent text-accent-foreground"
                : "text-foreground hover:bg-muted"
            )}
          >
            {file.is_dir ? (
              <Folder className="size-4 shrink-0 text-muted-foreground" />
            ) : (
              <FileText className="size-4 shrink-0 text-muted-foreground" />
            )}
            <span className="min-w-0 flex-1 truncate font-mono text-xs">
              {file.path}
            </span>
            {!file.is_dir && (
              <span className="shrink-0 text-[10px] text-muted-foreground">
                {formatSize(file.size)}
              </span>
            )}
          </div>
        ))}
        {files.length === 0 && !isSearching && (
          <div className="px-3 py-4 text-center text-xs text-muted-foreground">
            No matching files in workspace
          </div>
        )}
      </div>
    </div>
  )
}
