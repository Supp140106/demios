import { useState, useEffect, useRef } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { MessageCircleQuestion } from "lucide-react"

export type HumanInputRequest = {
  id: string
  question: string
  options?: string[]
}

type HumanInputDialogProps = {
  request: HumanInputRequest | null
  onSubmit: (id: string, input: string) => void
  onCancel: (id: string) => void
}

export function HumanInputDialog({
  request,
  onSubmit,
  onCancel,
}: HumanInputDialogProps) {
  const [input, setInput] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (request) {
      setInput("")
      setTimeout(() => inputRef.current?.focus(), 100)
    }
  }, [request])

  if (!request) return null

  const handleSubmit = (value: string) => {
    if (value.trim()) {
      onSubmit(request.id, value.trim())
    }
  }

  const handleCancel = () => {
    onCancel(request.id)
  }

  return (
    <AlertDialog open={true} onOpenChange={(open) => !open && handleCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <MessageCircleQuestion className="h-5 w-5 text-primary" />
            Input Needed
          </AlertDialogTitle>
          <AlertDialogDescription>
            <div className="space-y-4 pt-2">
              <p className="text-sm text-foreground">{request.question}</p>

              {request.options && request.options.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {request.options.map((opt) => (
                    <Button
                      key={opt}
                      variant="outline"
                      size="sm"
                      onClick={() => handleSubmit(opt)}
                    >
                      {opt}
                    </Button>
                  ))}
                </div>
              )}

              <div className="flex gap-2">
                <Input
                  ref={inputRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleSubmit(input)
                  }}
                  placeholder="Type your response..."
                  className="flex-1"
                />
                <Button
                  variant="default"
                  onClick={() => handleSubmit(input)}
                  disabled={!input.trim()}
                >
                  Send
                </Button>
              </div>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="ghost" onClick={handleCancel}>
            Cancel
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
