import { useState, useCallback, useRef } from "react"
import { cn } from "@/lib/utils"
import { Globe, RotateCw } from "lucide-react"

interface CanvasWebsiteProps {
  initialUrl?: string
  onNavigate?: (url: string) => void
}

export function CanvasWebsite({
  initialUrl = "about:blank",
  onNavigate,
}: CanvasWebsiteProps) {
  const [url, setUrl] = useState(initialUrl)
  const [loadedUrl, setLoadedUrl] = useState(initialUrl)
  const [loading, setLoading] = useState(false)
  const [showPrompt, setShowPrompt] = useState(
    !initialUrl || initialUrl === "about:blank"
  )
  const [promptUrl, setPromptUrl] = useState("")
  const iframeRef = useRef<HTMLIFrameElement>(null)

  const navigate = useCallback(
    (target: string) => {
      if (!target || target === "about:blank") return
      const normalized =
        target.startsWith("http://") || target.startsWith("https://")
          ? target
          : `https://${target}`
      setUrl(normalized)
      setLoadedUrl(normalized)
      setLoading(true)
      setShowPrompt(false)
      onNavigate?.(normalized)
    },
    [onNavigate]
  )

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") {
        navigate(url)
      }
    },
    [url, navigate]
  )

  const handlePromptKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" && promptUrl.trim()) {
        navigate(promptUrl.trim())
      }
    },
    [promptUrl, navigate]
  )

  if (showPrompt) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-4 bg-muted/20">
        <Globe className="h-12 w-12 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">Enter a URL to open</p>
        <div className="flex w-full max-w-md items-center gap-2">
          <input
            value={promptUrl}
            onChange={(e) => setPromptUrl(e.target.value)}
            onKeyDown={handlePromptKeyDown}
            placeholder="https://example.com"
            className="flex-1 rounded-md border bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
            autoFocus
          />
          <button
            onClick={() => navigate(promptUrl.trim())}
            disabled={!promptUrl.trim()}
            className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-50"
          >
            Go
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full w-full flex-col">
      <div className="flex items-center gap-2 border-b bg-muted/30 px-2 py-1.5">
        <button
          onClick={() => navigate(url)}
          className="rounded p-1 text-muted-foreground hover:bg-muted"
        >
          <RotateCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
        </button>
        <div className="flex flex-1 items-center gap-2 rounded-md bg-background px-3 py-1">
          <Globe className="h-3 w-3 shrink-0 text-muted-foreground" />
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={handleKeyDown}
            className="flex-1 bg-transparent text-xs outline-none"
          />
        </div>
      </div>
      <div className="flex-1">
        <iframe
          ref={iframeRef}
          src={loadedUrl}
          className="h-full w-full border-0"
          onLoad={() => setLoading(false)}
          sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
        />
      </div>
    </div>
  )
}
