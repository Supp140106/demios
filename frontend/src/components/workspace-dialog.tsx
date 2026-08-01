import { useState } from "react"
import { Button } from "@/components/ui/button"
import { FolderOpen } from "lucide-react"
import { PickDirectory } from "../../wailsjs/go/main/App"

type WorkspaceDialogProps = {
  onSelect: (path: string) => void
}

export function WorkspaceDialog({ onSelect }: WorkspaceDialogProps) {
  const [inputValue, setInputValue] = useState("")

  const handleBrowse = async () => {
    const path = await PickDirectory()
    if (path) {
      onSelect(path)
    }
  }

  const handleSubmit = () => {
    const trimmed = inputValue.trim()
    if (trimmed) {
      onSelect(trimmed)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="mx-4 w-full max-w-md rounded-2xl border bg-card p-6 shadow-lg">
        <div className="mb-2 flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10">
            <FolderOpen className="h-5 w-5 text-primary" />
          </div>
          <div>
            <h2 className="text-lg font-semibold">Select Workspace</h2>
            <p className="text-sm text-muted-foreground">
              Choose a folder for the AI agent to work in
            </p>
          </div>
        </div>

        <div className="mt-4 flex gap-2">
          <div className="relative flex-1">
            <input
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleSubmit()
              }}
              placeholder="C:\\Code\\my-project"
              className="w-full rounded-xl border bg-background px-4 py-2.5 pr-10 text-sm ring-offset-background outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
            />
            <button
              type="button"
              onClick={handleBrowse}
              className="absolute top-1/2 right-2.5 -translate-y-1/2 cursor-pointer rounded-md p-1 text-muted-foreground transition-colors hover:text-foreground"
              title="Browse folders"
            >
              <FolderOpen className="h-4 w-4" />
            </button>
          </div>
          <Button onClick={handleSubmit} size="default">
            Set
          </Button>
        </div>

        <p className="mt-2 text-xs text-muted-foreground">
          Type a path or click the folder icon to browse
        </p>
      </div>
    </div>
  )
}
