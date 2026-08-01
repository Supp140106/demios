import { Select } from "@/components/ui/select"
import type { ModelInfo } from "@/hooks/use-model"

type ModelSelectorProps = {
  models: ModelInfo[]
  currentModel: string
  onSelect: (id: string) => void
}

export function ModelSelector({
  models,
  currentModel,
  onSelect,
}: ModelSelectorProps) {
  if (models.length === 0) return null

  return (
    <Select.Root
      value={currentModel}
      onValueChange={(value) => {
        if (value !== null) onSelect(value)
      }}
    >
      <Select.Trigger>
        <Select.Value />
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
