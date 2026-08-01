import { ChatMessage } from "@/components/chat-message"
import { ChatContainerScrollAnchor } from "@/components/ui/chat-container"
import { Loader } from "@/components/ui/loader"
import type { Message } from "@/hooks/use-chat-state"

type ChatInterfaceProps = {
  messages: Message[]
  isLoading: boolean
}

export function ChatInterface({ messages, isLoading }: ChatInterfaceProps) {
  return (
    <div className="mx-auto w-full max-w-3xl px-4 pt-8 pb-4">
      {messages.map((msg) => (
        <div key={msg.id} className="mb-6 last:mb-0">
          <ChatMessage message={msg} />
        </div>
      ))}
      {isLoading && (
        <div className="flex items-center gap-2 px-4 py-2 text-muted-foreground">
          <Loader variant="typing" size="sm" />
          <span className="text-xs">Thinking...</span>
        </div>
      )}
      <ChatContainerScrollAnchor />
    </div>
  )
}
