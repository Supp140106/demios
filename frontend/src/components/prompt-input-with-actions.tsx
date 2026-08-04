import {
  PromptInput,
  PromptInputAction,
  PromptInputActions,
  PromptInputTextarea,
} from "@/components/ui/prompt-input"
import { ModelSelector } from "@/components/model-selector"
import type { ModelInfo } from "@/hooks/use-model"
import { cn } from "@/lib/utils"
import { ArrowUp, Plus, SlidersHorizontal, Square, X } from "lucide-react"
import { useRef, useState } from "react"

type Attachment = {
  id: string
  name: string
  url: string
}

type PromptInputWithActionsProps = {
  onSend: (message: string) => void
  onStop: () => void
  isLoading: boolean
  workspaceMissing: boolean
  onOpenProviders?: () => void
  models: ModelInfo[]
  currentModel: string
  onModelSelect: (id: string) => void
}

export function PromptInputWithActions({
  onSend,
  onStop,
  isLoading,
  workspaceMissing,
  onOpenProviders,
  models,
  currentModel,
  onModelSelect,
}: PromptInputWithActionsProps) {
  const [input, setInput] = useState("")
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)

  const canSend = input.trim().length > 0 && !isLoading && !workspaceMissing

  const handleSubmit = () => {
    if (!canSend) return
    onSend(input)
    setInput("")
    setAttachments((prev) => {
      prev.forEach((a) => URL.revokeObjectURL(a.url))
      return []
    })
  }

  const handleFiles = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? []).filter((f) =>
      f.type.startsWith("image/")
    )
    e.target.value = ""
    for (const file of files) {
      setAttachments((prev) => [
        ...prev,
        {
          id: `${file.name}-${file.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
          name: file.name,
          url: URL.createObjectURL(file),
        },
      ])
    }
  }

  const removeAttachment = (id: string) => {
    setAttachments((prev) => {
      const target = prev.find((a) => a.id === id)
      if (target) URL.revokeObjectURL(target.url)
      return prev.filter((a) => a.id !== id)
    })
  }

  return (
    <div className="mx-auto w-full max-w-3xl">
      <PromptInput
        value={input}
        onValueChange={setInput}
        isLoading={isLoading}
        onSubmit={handleSubmit}
        disabled={workspaceMissing}
        className="rounded-[26px] border-border/80 bg-card/70 p-2 shadow-[0_1px_2px_rgba(0,0,0,0.04),0_8px_28px_-14px_rgba(0,0,0,0.18)] backdrop-blur transition-colors focus-within:border-ring/50 focus-within:ring-2 focus-within:ring-ring/15"
      >
        {attachments.length > 0 && (
          <div className="flex flex-wrap gap-2 px-1.5 pt-1">
            {attachments.map((a) => (
              <div
                key={a.id}
                className="group relative size-16 overflow-hidden rounded-xl border border-border"
              >
                <img
                  src={a.url}
                  alt={a.name}
                  className="size-full object-cover"
                />
                <button
                  type="button"
                  onClick={() => removeAttachment(a.id)}
                  className="absolute top-0.5 right-0.5 flex size-4.5 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
                  aria-label={`Remove ${a.name}`}
                >
                  <X className="size-3" />
                </button>
              </div>
            ))}
          </div>
        )}

        <PromptInputTextarea
          placeholder={
            workspaceMissing
              ? "Select a workspace folder to start..."
              : "Message Demios..."
          }
          disabled={workspaceMissing}
          className="min-h-[26px] px-3 py-2.5 text-[15px] leading-relaxed"
        />

        <PromptInputActions className="flex items-center justify-between gap-2 px-1.5 pt-1">
          <div className="flex items-center gap-0.5">
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              multiple
              className="hidden"
              tabIndex={-1}
              aria-hidden="true"
              onChange={handleFiles}
            />
            <PromptInputAction tooltip="Add attachments">
              <span
                onClick={() => fileInputRef.current?.click()}
                className="flex size-8 cursor-pointer items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              >
                <Plus className="size-4" />
              </span>
            </PromptInputAction>

            <ModelSelector
              models={models}
              currentModel={currentModel}
              onSelect={onModelSelect}
            />

            {onOpenProviders && (
              <span
                onClick={onOpenProviders}
                className="flex size-8 cursor-pointer items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                title="LLM providers"
                aria-label="LLM providers"
              >
                <SlidersHorizontal className="size-3.5" />
              </span>
            )}
          </div>

          <PromptInputAction
            tooltip={isLoading ? "Stop generation" : "Send message"}
          >
            <span
              onClick={isLoading ? onStop : handleSubmit}
              className={cn(
                "flex size-8 cursor-pointer items-center justify-center rounded-full transition-all",
                "disabled:pointer-events-none disabled:opacity-40",
                canSend
                  ? "bg-primary text-primary-foreground shadow-sm hover:bg-primary/85"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              )}
              aria-disabled={!canSend}
            >
              {isLoading ? (
                <Square className="size-3.5 fill-current" />
              ) : (
                <ArrowUp className="size-4" />
              )}
            </span>
          </PromptInputAction>
        </PromptInputActions>
      </PromptInput>
    </div>
  )
}
