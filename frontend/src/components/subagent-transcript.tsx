import { useEffect, useRef } from "react"
import { Tool } from "@/components/ui/tool"
import type { SubagentTranscript as Transcript } from "@/hooks/use-chat-state"
import { Bot, Loader2 } from "lucide-react"

type SubagentTranscriptProps = {
  transcript: Transcript
}

export function SubagentTranscript({ transcript }: SubagentTranscriptProps) {
  const scrollRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const el = scrollRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [
    transcript.thinking,
    transcript.content,
    transcript.toolCalls,
    transcript.screenshot,
  ])

  return (
    <div className="overflow-hidden rounded-lg border bg-muted/30">
      <div className="flex items-center gap-2 border-b px-2.5 py-1.5">
        <Bot className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="text-xs font-medium">Browser Agent</span>
        {transcript.url && (
          <span className="truncate text-xs text-muted-foreground">
            {transcript.title || transcript.url}
          </span>
        )}
        {transcript.active && (
          <span className="ml-auto flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
            <Loader2 className="h-3 w-3 animate-spin" />
            Working
          </span>
        )}
      </div>

      <div
        ref={scrollRef}
        className="max-h-72 space-y-2 overflow-auto p-2.5 text-xs"
      >
        {transcript.thinking && (
          <div className="whitespace-pre-wrap text-muted-foreground">
            <span className="font-medium text-foreground">Thinking: </span>
            {transcript.thinking}
          </div>
        )}

        {transcript.toolCalls.map((tc) => (
          <Tool
            key={tc.id}
            toolPart={{
              type: tc.name,
              state:
                tc.status === "running"
                  ? "input-streaming"
                  : tc.status === "completed"
                    ? "output-available"
                    : tc.status === "error"
                      ? "output-error"
                      : "input-available",
              input: tc.args as Record<string, unknown>,
              output: tc.output ? { result: tc.output } : undefined,
              errorText: tc.error,
              toolCallId: tc.id,
            }}
            defaultOpen={tc.status === "running"}
          />
        ))}

        {transcript.screenshot && (
          <div>
            <div className="mb-1 text-xs text-muted-foreground">Screenshot</div>
            <img
              src={`data:image/png;base64,${transcript.screenshot}`}
              alt="Browser screenshot"
              className="h-auto w-full rounded border object-cover"
            />
          </div>
        )}

        {transcript.content && (
          <div className="whitespace-pre-wrap text-muted-foreground">
            {transcript.content}
          </div>
        )}

        {transcript.error && (
          <div className="rounded border border-red-200 bg-red-50 p-2 text-red-600 dark:border-red-950 dark:bg-red-950/40 dark:text-red-400">
            {transcript.error}
          </div>
        )}
      </div>
    </div>
  )
}
