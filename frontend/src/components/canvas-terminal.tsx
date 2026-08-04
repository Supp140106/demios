import { useEffect, useRef, useState, useCallback } from "react"
import { Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import { Unicode11Addon } from "@xterm/addon-unicode11"
import "@xterm/xterm/css/xterm.css"
import {
  Terminal as TerminalIcon,
  Plus,
  X,
  Circle,
} from "lucide-react"
import {
  CreateTerminal,
  WriteTerminal,
  ReadTerminal,
  CloseTerminal,
  ListTerminals,
  ResizeTerminal,
} from "../../wailsjs/go/main/App"

interface TerminalTab {
  id: string
  title: string
  status: "active" | "closed"
}

interface CanvasTerminalProps {
  /** Called when a new terminal is created so the parent can track it */
  onTerminalCreated?: (id: string) => void
}

export function CanvasTerminal({ onTerminalCreated }: CanvasTerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const activeIdRef = useRef<string | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const [tabs, setTabs] = useState<TerminalTab[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)

  // Initialize xterm.js once
  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: "bar",
      fontSize: 13,
      fontFamily:
        "'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'SF Mono', monospace",
      theme: {
        background: "#0a0a0a",
        foreground: "#e4e4e7",
        cursor: "#71717a",
        selectionBackground: "#27272a",
        black: "#18181b",
        red: "#ef4444",
        green: "#22c55e",
        yellow: "#eab308",
        blue: "#3b82f6",
        magenta: "#a855f7",
        cyan: "#06b6d4",
        white: "#e4e4e7",
        brightBlack: "#3f3f46",
        brightRed: "#f87171",
        brightGreen: "#4ade80",
        brightYellow: "#facc15",
        brightBlue: "#60a5fa",
        brightMagenta: "#c084fc",
        brightCyan: "#22d3ee",
        brightWhite: "#fafafa",
      },
      allowProposedApi: true,
      scrollback: 10000,
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)

    const unicodeAddon = new Unicode11Addon()
    term.loadAddon(unicodeAddon)

    term.open(containerRef.current)
    fitAddon.fit()

    // Handle user keyboard input → send to backend PTY
    term.onData((data) => {
      const id = activeIdRef.current
      if (!id) return
      WriteTerminal(id, data).catch(() => {})
    })

    termRef.current = term
    fitAddonRef.current = fitAddon

    return () => {
      term.dispose()
      termRef.current = null
      fitAddonRef.current = null
    }
  }, [])

  // Sync activeIdRef with state
  useEffect(() => {
    activeIdRef.current = activeId
  }, [activeId])

  // Resize handler
  useEffect(() => {
    const handleResize = () => {
      fitAddonRef.current?.fit()
      // Notify backend of new size
      if (activeId && termRef.current) {
        const dims = termRef.current.cols + "x" + termRef.current.rows
        const [cols, rows] = dims.split("x").map(Number)
        ResizeTerminal(activeId, cols, rows).catch(() => {})
      }
    }

    window.addEventListener("resize", handleResize)
    const observer = new ResizeObserver(handleResize)
    if (containerRef.current) {
      observer.observe(containerRef.current)
    }

    return () => {
      window.removeEventListener("resize", handleResize)
      observer.disconnect()
    }
  }, [activeId])

  // Poll for terminal output
  useEffect(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
    }

    if (!activeId) {
      termRef.current?.clear()
      return
    }

    pollRef.current = setInterval(async () => {
      try {
        const output = await ReadTerminal(activeId)
        if (output && termRef.current) {
          termRef.current.write(output)
        }
      } catch {
        // session may have been closed
      }
    }, 50) // 50ms poll for responsive feel

    return () => {
      if (pollRef.current) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
    }
  }, [activeId])

  // Clear terminal when switching sessions
  useEffect(() => {
    if (termRef.current && activeId) {
      termRef.current.clear()
    }
  }, [activeId])

  const createTerminal = useCallback(async () => {
    setCreating(true)
    try {
      const info = await CreateTerminal("", "")
      const newTab: TerminalTab = {
        id: info.id,
        title: `Terminal ${tabs.length + 1}`,
        status: "active",
      }
      setTabs((prev) => [...prev, newTab])
      setActiveId(info.id)
      onTerminalCreated?.(info.id)
    } catch (err) {
      console.error("Failed to create terminal:", err)
    } finally {
      setCreating(false)
    }
  }, [tabs.length, onTerminalCreated])

  const closeTerminal = useCallback(
    async (id: string) => {
      await CloseTerminal(id)
      setTabs((prev) => {
        const next = prev.map((t) =>
          t.id === id ? { ...t, status: "closed" as const } : t
        )
        // If we closed the active tab, switch to another
        if (activeId === id) {
          const firstActive = next.find((t) => t.status === "active")
          setActiveId(firstActive?.id || null)
        }
        return next
      })
    },
    [activeId]
  )

  // Load existing terminals on mount
  useEffect(() => {
    ListTerminals().then((ids) => {
      if (ids.length > 0) {
        const existing: TerminalTab[] = ids.map((id, i) => ({
          id,
          title: `Terminal ${i + 1}`,
          status: "active" as const,
        }))
        setTabs(existing)
        setActiveId(ids[0])
      }
    }).catch(() => {})
  }, [])

  return (
    <div className="flex h-full w-full flex-col bg-[#0a0a0a]">
      {/* Tab bar */}
      <div className="flex items-center border-b border-[#27272a] bg-[#18181b]">
        <div className="flex flex-1 items-center overflow-x-auto">
          {tabs.map((tab) => (
            <div
              key={tab.id}
              onClick={() => tab.status === "active" && setActiveId(tab.id)}
              className={`group flex cursor-pointer items-center gap-1.5 border-r border-[#27272a] px-3 py-1.5 text-xs ${
                activeId === tab.id
                  ? "bg-[#0a0a0a] text-foreground"
                  : "text-muted-foreground hover:bg-[#27272a]/50"
              }`}
            >
              <Circle
                className={`h-2 w-2 ${
                  tab.status === "active"
                    ? "fill-green-500 text-green-500"
                    : "fill-zinc-600 text-zinc-600"
                }`}
              />
              <span className="max-w-24 truncate">{tab.title}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  closeTerminal(tab.id)
                }}
                className="ml-1 rounded-sm p-0.5 opacity-0 group-hover:opacity-100 hover:bg-[#27272a]"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>

        <button
          onClick={createTerminal}
          disabled={creating}
          className="rounded p-1.5 text-muted-foreground hover:bg-[#27272a] disabled:opacity-50"
        >
          <Plus className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* Terminal container */}
      <div ref={containerRef} className="flex-1" />

      {/* Empty state */}
      {tabs.length === 0 && (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-3">
          <TerminalIcon className="h-12 w-12 text-zinc-700" />
          <p className="text-sm text-zinc-500">No terminal sessions</p>
          <button
            onClick={createTerminal}
            disabled={creating}
            className="rounded-md bg-[#27272a] px-4 py-2 text-sm text-foreground hover:bg-[#3f3f46] disabled:opacity-50"
          >
            {creating ? "Creating..." : "New Terminal"}
          </button>
        </div>
      )}
    </div>
  )
}
