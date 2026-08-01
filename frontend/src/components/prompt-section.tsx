import { PromptInput } from "@/components/ui/ai-chat-input"

type PromptSectionProps = {
  models: { ID: string; Label: string }[]
  currentModel: string
  onSelectModel: (id: string) => void
  onSubmit: (content: string) => void
  browserActive: boolean
}

export function PromptSection({
  models,
  currentModel,
  onSelectModel,
  onSubmit,
  browserActive,
}: PromptSectionProps) {
  return (
    <div className="flex w-full justify-center px-4 pb-4">
      <PromptInput
        onSubmit={(value, meta) => {
          const matched = models.find((m) => m.Label === meta.model)
          if (matched && matched.ID !== currentModel) onSelectModel(matched.ID)
          onSubmit(value)
        }}
        placeholder={
          browserActive
            ? "Agent is controlling the browser... Type a message or @browser to start a new task"
            : "Ask me to build something..."
        }
        models={models.map((m) => m.Label)}
      />
      {browserActive && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 bg-primary/10 border border-primary/30 rounded-full px-4 py-1 text-xs text-primary font-medium">
          Browser Agent Active
        </div>
      )}
    </div>
  )
}
