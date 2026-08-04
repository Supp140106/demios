import { Select } from "@/components/ui/select"
import type { ModelInfo } from "@/hooks/use-model"
import { cn } from "@/lib/utils"
import { Sparkles } from "lucide-react"

type ModelSelectorProps = {
  models: ModelInfo[]
  currentModel: string
  onSelect: (id: string) => void
  className?: string
}

export function ModelSelector({
  models,
  currentModel,
  onSelect,
  className,
}: ModelSelectorProps) {
  if (models.length === 0) return null

  return (
    <Select.Root
      value={currentModel}
      onValueChange={(value) => {
        if (value !== null) onSelect(value)
      }}
    >
      <Select.Trigger
        className={cn(
          "h-8 rounded-full border-0 bg-muted/60 px-3 text-xs font-medium text-foreground",
          "hover:bg-muted focus:ring-1 focus:ring-ring/40",
          className
        )}
      >
        <Sparkles className="h-3.5 w-3.5 shrink-0 text-primary" />
        <Select.Value className="max-w-32 text-xs font-medium" />
      </Select.Trigger>
      <Select.Popup>
        <Select.List>
          {models.map((m) => (
            <Select.Item key={m.ID} value={m.ID}>
              {m.Label}
            </Select.Item>
          ))}
        </Select.List>
      </Select.Popup>
    </Select.Root>
  )
}
