import { PromptInputWithActions } from "@/components/prompt-input-with-actions"
import type { ModelInfo } from "@/hooks/use-model"

type PromptSectionProps = {
  models: ModelInfo[]
  currentModel: string
  onSelectModel: (id: string) => void
  onSubmit: (content: string) => void
  browserActive: boolean
  isLoading: boolean
  onStop: () => void
  workspaceMissing: boolean
  onOpenProviders?: () => void
}

export function PromptSection({
  models,
  currentModel,
  onSelectModel,
  onSubmit,
  browserActive,
  isLoading,
  onStop,
  workspaceMissing,
  onOpenProviders,
}: PromptSectionProps) {
  return (
    <div className="flex w-full justify-center px-4 pb-4">
      <PromptInputWithActions
        onSend={onSubmit}
        onStop={onStop}
        isLoading={isLoading}
        workspaceMissing={workspaceMissing}
        onOpenProviders={onOpenProviders}
        models={models}
        currentModel={currentModel}
        onModelSelect={onSelectModel}
      />
      {browserActive && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-primary/30 bg-primary/10 px-4 py-1 text-xs font-medium text-primary">
          Browser Agent Active
        </div>
      )}
    </div>
  )
}
