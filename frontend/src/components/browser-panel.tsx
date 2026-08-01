import { Button } from "@/components/ui/button"
import { useChatState } from "@/hooks/use-chat-state"
import { Eye, Trash2 } from "lucide-react"

export function BrowserPanel() {
  const { browserState, stopBrowser, takeBrowserControl, giveBrowserControl } =
    useChatState()

  if (!browserState.active) return null

  return (
    <div className="flex flex-col border-b bg-muted/30">
      <div className="flex items-center gap-2 px-3 py-2 border-b">
        <Eye className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-xs font-medium text-muted-foreground">
          Browser Agent
        </span>
        <span className="ml-auto text-xs text-muted-foreground">
          {browserState.mode === "user" ? "User Control" : "Agent Control"}
        </span>
      </div>

      <div className="flex items-center gap-1 px-3 py-1.5">
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={() => window.open(browserState.url, "_blank")}
          title="Open in browser"
          className="h-6 w-6"
        >
          <Eye className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={stopBrowser}
          title="Stop browser"
          className="h-6 w-6"
        >
          <Trash2 className="h-3 w-3" />
        </Button>
        <div className="flex-1" />
        {browserState.mode === "agent" ? (
          <Button
            variant="outline"
            size="sm"
            onClick={takeBrowserControl}
            className="h-6 text-xs"
          >
            Take Control
          </Button>
        ) : (
          <Button
            variant="default"
            size="sm"
            onClick={giveBrowserControl}
            className="h-6 text-xs"
          >
            Give Control
          </Button>
        )}
      </div>
    </div>
  )
}