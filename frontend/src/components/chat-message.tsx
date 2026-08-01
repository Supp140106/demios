import { Message, MessageContent } from "@/components/ui/message"
import {
  Reasoning,
  ReasoningTrigger,
  ReasoningContent,
} from "@/components/ui/reasoning"
import { Tool } from "@/components/ui/tool"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { DiffViewer } from "@/components/diff-viewer"
import { Bot, User, Brain, FileDiff } from "lucide-react"
import type { Message as MessageType } from "@/hooks/use-chat-state"

type ChatMessageProps = {
  message: MessageType
}

export function ChatMessage({ message }: ChatMessageProps) {
  const isUser = message.role === "user"

  return (
    <Message className={isUser ? "flex-row-reverse" : ""}>
      <Avatar className="h-8 w-8 shrink-0">
        <AvatarFallback
          className={
            isUser
              ? "bg-primary text-primary-foreground"
              : "bg-muted text-muted-foreground"
          }
        >
          {isUser ? (
            <User className="h-3.5 w-3.5" />
          ) : (
            <Bot className="h-3.5 w-3.5" />
          )}
        </AvatarFallback>
      </Avatar>
      <div className="flex min-w-0 flex-col gap-2">
        {message.thinking && (
          <Reasoning>
            <ReasoningTrigger className="text-xs">
              <Brain className="h-3 w-3" />
              Reasoning
            </ReasoningTrigger>
            <ReasoningContent markdown className="text-xs">
              {message.thinking}
            </ReasoningContent>
          </Reasoning>
        )}

        {message.toolCalls.length > 0 && (
          <div className="flex flex-col gap-1.5">
            {message.toolCalls.map((tc) => (
              <div key={tc.id} className="flex flex-col gap-1">
                <Tool
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
                  defaultOpen={false}
                />
                {tc.diff && (
                  <div className="ml-6">
                    <div className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-muted-foreground">
                      <FileDiff className="h-3.5 w-3.5" />
                      <span>Diff</span>
                    </div>
                    <DiffViewer
                      patch={tc.diff}
                      variant="muted"
                      size="sm"
                      viewMode="unified"
                    />
                  </div>
                )}
                {tc.diffs && tc.diffs.length > 0 && (
                  <div className="ml-6 space-y-2">
                    <div className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-muted-foreground">
                      <FileDiff className="h-3.5 w-3.5" />
                      <span>
                        Changes ({tc.diffs.length} file
                        {tc.diffs.length !== 1 ? "s" : ""})
                      </span>
                    </div>
                    {tc.diffs.map((d, i) => (
                      <DiffViewer
                        key={i}
                        patch={d.patch}
                        variant="muted"
                        size="sm"
                        viewMode="unified"
                      />
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {message.content && (
          <MessageContent markdown>{message.content}</MessageContent>
        )}
      </div>
    </Message>
  )
}
