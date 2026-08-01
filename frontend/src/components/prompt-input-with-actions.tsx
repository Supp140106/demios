import {
  PromptInput,
  PromptInputAction,
  PromptInputActions,
  PromptInputTextarea,
} from "@/components/ui/prompt-input"
import { ModelSelector } from "@/components/model-selector"
import type { ModelInfo } from "@/hooks/use-model"
import { ArrowUp, Square, FolderOpen } from "lucide-react"
import { useState } from "react"

type PromptInputWithActionsProps = {
  onSend: (message: string) => void
  onStop: () => void
  isLoading: boolean
  workspaceMissing: boolean
  onOpenWorkspace: () => void
  models: ModelInfo[]
  currentModel: string
  onModelSelect: (id: string) => void
}

export function PromptInputWithActions({
  onSend,
  onStop,
  isLoading,
  workspaceMissing,
  onOpenWorkspace,
  models,
  currentModel,
  onModelSelect,
}: PromptInputWithActionsProps) {
  const [input, setInput] = useState("")

  const handleSubmit = () => {
    if (input.trim() && !isLoading && !workspaceMissing) {
      onSend(input)
      setInput("")
    }
  }

  return (
    <PromptInput
      value={input}
      onValueChange={setInput}
      isLoading={isLoading}
      onSubmit={handleSubmit}
      disabled={workspaceMissing}
      className="mx-auto w-full max-w-3xl"
    >
      <PromptInputTextarea
        placeholder={
          workspaceMissing
            ? "Select a workspace folder to start..."
            : "Ask me to build something..."
        }
        disabled={workspaceMissing}
      />

      <PromptInputActions className="flex items-center justify-between gap-2 pt-2">
        <div className="flex items-center gap-1">
          <PromptInputAction tooltip="Select workspace">
            <span
              onClick={onOpenWorkspace}
              className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-2xl hover:bg-secondary-foreground/10"
            >
              <FolderOpen className="h-5 w-5 text-primary" />
            </span>
          </PromptInputAction>

          <ModelSelector
            models={models}
            currentModel={currentModel}
            onSelect={onModelSelect}
          />
        </div>

        <PromptInputAction
          tooltip={isLoading ? "Stop generation" : "Send message"}
        >
          <span
            onClick={isLoading ? onStop : handleSubmit}
            className="inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-primary text-primary-foreground hover:bg-primary/80"
          >
            {isLoading ? (
              <Square className="h-5 w-5 fill-current" />
            ) : (
              <ArrowUp className="h-5 w-5" />
            )}
          </span>
        </PromptInputAction>
      </PromptInputActions>
    </PromptInput>
  )
}
