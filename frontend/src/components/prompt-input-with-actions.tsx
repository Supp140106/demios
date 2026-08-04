import {
  PromptInput,
  PromptInputAction,
  PromptInputActions,
  PromptInputTextarea,
} from "@/components/ui/prompt-input"
import { FileMentionPopover } from "@/components/file-mention-popover"
import { ModelSelector } from "@/components/model-selector"
import type { ModelInfo } from "@/hooks/use-model"
import { useFileSearch } from "@/hooks/use-file-search"
import { cn } from "@/lib/utils"
import { ArrowUp, Plus, SlidersHorizontal, Square, X } from "lucide-react"
import { useRef, useState, useCallback, useEffect } from "react"

type Attachment = {
  id: string
  name: string
  url: string
}

type MentionState = {
  active: boolean
  query: string
  startIndex: number
  highlightedIndex: number
}

type PromptInputWithActionsProps = {
  onSend: (message: string, fileContents?: Record<string, string>) => void
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
  const [mention, setMention] = useState<MentionState>({
    active: false,
    query: "",
    startIndex: -1,
    highlightedIndex: 0,
  })
  const [popoverPos, setPopoverPos] = useState<{
    top: number
    left: number
  } | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const { results, isSearching, search, reset } = useFileSearch()
  const mentionRef = useRef(mention)
  const resultsRef = useRef(results)

  useEffect(() => {
    mentionRef.current = mention
    resultsRef.current = results
  }, [mention, results])

  const canSend = input.trim().length > 0 && !isLoading && !workspaceMissing

  const insertFileToken = useCallback(
    (filePath: string) => {
      const m = mentionRef.current
      if (!m.active) return
      const before = input.slice(0, m.startIndex)
      const after = input.slice(m.startIndex + m.query.length + 1)
      const token = `@${filePath}`
      const next = before + token + " " + after
      setInput(next)
      setMention({
        active: false,
        query: "",
        startIndex: -1,
        highlightedIndex: 0,
      })
      reset()
      setTimeout(() => {
        const ta = textareaRef.current
        if (!ta) return
        const pos = m.startIndex + token.length + 1
        ta.setSelectionRange(pos, pos)
        ta.focus()
      }, 0)
    },
    [input, reset]
  )

  const handleInputChange = useCallback(
    (value: string) => {
      setInput(value)

      const ta = textareaRef.current
      if (!ta) return

      const cursorPos = ta.selectionStart
      const textBeforeCursor = value.slice(0, cursorPos)

      const lastAt = textBeforeCursor.lastIndexOf("@")
      const lastSpace = Math.max(
        textBeforeCursor.lastIndexOf(" "),
        textBeforeCursor.lastIndexOf("\n")
      )

      if (lastAt > lastSpace && lastAt !== -1) {
        const query = textBeforeCursor.slice(lastAt + 1)
        if (!query.includes(" ") || query.length < 30) {
          const rect = ta.getBoundingClientRect()
          setMention({
            active: true,
            query,
            startIndex: lastAt,
            highlightedIndex: 0,
          })
          setPopoverPos({
            top: 60,
            left: Math.max(16, rect.left - 60),
          })
          search(query)
          return
        }
      }

      if (mentionRef.current.active) {
        setMention({
          active: false,
          query: "",
          startIndex: -1,
          highlightedIndex: 0,
        })
        setPopoverPos(null)
        reset()
      }
    },
    [search, reset]
  )

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (!mentionRef.current.active) return

      if (e.key === "ArrowDown") {
        e.preventDefault()
        setMention((prev) => ({
          ...prev,
          highlightedIndex: Math.min(
            prev.highlightedIndex + 1,
            results.length - 1
          ),
        }))
      } else if (e.key === "ArrowUp") {
        e.preventDefault()
        setMention((prev) => ({
          ...prev,
          highlightedIndex: Math.max(prev.highlightedIndex - 1, 0),
        }))
      } else if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault()
        if (results.length > 0) {
          insertFileToken(results[mentionRef.current.highlightedIndex].path)
        }
      } else if (e.key === "Tab") {
        e.preventDefault()
        if (results.length > 0) {
          insertFileToken(results[mentionRef.current.highlightedIndex].path)
        }
      } else if (e.key === "Escape") {
        e.preventDefault()
        setMention({
          active: false,
          query: "",
          startIndex: -1,
          highlightedIndex: 0,
        })
        setPopoverPos(null)
        reset()
      } else if (e.key === "Backspace") {
        const ta = e.currentTarget
        const cursorPos = ta.selectionStart
        const textBeforeCursor = input.slice(0, cursorPos)
        const lastAt = textBeforeCursor.lastIndexOf("@")
        if (
          lastAt !== -1 &&
          mentionRef.current.startIndex === lastAt &&
          mentionRef.current.query === ""
        ) {
          setMention({
            active: false,
            query: "",
            startIndex: -1,
            highlightedIndex: 0,
          })
          setPopoverPos(null)
          reset()
        }
      }
    },
    [results, insertFileToken, input, reset]
  )

  const handleSubmit = async () => {
    if (!canSend) return

    const mentionPattern = /@([\w./\\-]+)/g
    const matches = [...input.matchAll(mentionPattern)]

    if (matches.length === 0) {
      onSend(input)
    } else {
      const fileContents: Record<string, string> = {}
      const { ReadWorkspaceFile, ReadWorkspaceFolder } =
        await import("../../wailsjs/go/main/App")

      for (const match of matches) {
        const path = match[1]
        const entry = resultsRef.current.find((r) => r.path === path)
        if (entry?.is_dir) {
          try {
            const files = await ReadWorkspaceFolder(path, 30)
            for (const f of files) {
              fileContents[f.path] = f.content
            }
          } catch {
            // skip unreadable folders
          }
        } else {
          try {
            const content = await ReadWorkspaceFile(path)
            fileContents[path] = content
          } catch {
            // skip unreadable files
          }
        }
      }
      onSend(input, fileContents)
    }

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
      <div className="relative">
        <FileMentionPopover
          files={results}
          isSearching={isSearching}
          isOpen={mention.active}
          query={mention.query}
          highlightedIndex={mention.highlightedIndex}
          onSelect={(file) => insertFileToken(file.path)}
          position={popoverPos}
        />

        <PromptInput
          value={input}
          onValueChange={handleInputChange}
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
            ref={textareaRef}
            onKeyDown={handleKeyDown}
            placeholder={
              workspaceMissing
                ? "Select a workspace folder to start..."
                : "Message Demios... (@ to mention files or folders)"
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
    </div>
  )
}
