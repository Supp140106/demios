import { useState, useCallback, useRef, useEffect } from "react"
import { AgentTopology } from "@/components/agent-topology"
import { CanvasWebsite } from "@/components/canvas-website"
import { CanvasTerminal } from "@/components/canvas-terminal"
import type { TopologyState } from "@/hooks/use-chat-state"

import { cn } from "@/lib/utils"
import { X, Plus, CircuitBoard, Globe, Terminal } from "lucide-react"

type TabType = "topology" | "website" | "terminal"

interface Tab {
  id: string
  title: string
  type: TabType
  url?: string
}

let tabCounter = 0

interface CanvasPanelProps {
  topologyState?: TopologyState
}

export function CanvasPanel({ topologyState }: CanvasPanelProps) {
  const [tabs, setTabs] = useState<Tab[]>([
    { id: "tab-0", title: "Topology", type: "topology" },
  ])
  const [activeTab, setActiveTab] = useState("tab-0")
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menuOpen) return
    const handler = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    document.addEventListener("mousedown", handler)
    return () => document.removeEventListener("mousedown", handler)
  }, [menuOpen])

  const addTab = useCallback((type: TabType, title: string, url?: string) => {
    tabCounter++
    const id = `tab-${tabCounter}`
    setTabs((prev) => [...prev, { id, title, type, url }])
    setActiveTab(id)
    setMenuOpen(false)
  }, [])

  const closeTab = useCallback(
    (id: string) => {
      setTabs((prev) => {
        const next = prev.filter((t) => t.id !== id)
        if (next.length === 0) {
          tabCounter++
          return [
            { id: `tab-${tabCounter}`, title: "Topology", type: "topology" },
          ]
        }
        return next
      })
      setActiveTab((prev) => {
        if (prev === id) {
          const idx = tabs.findIndex((t) => t.id === id)
          return tabs[Math.max(0, idx - 1)]?.id ?? tabs[0]?.id ?? ""
        }
        return prev
      })
    },
    [tabs]
  )

  const active = tabs.find((t) => t.id === activeTab)

  return (
    <div className="flex h-full w-full flex-col bg-background">
      <div className="flex items-center border-b bg-muted/20">
        <div className="flex flex-1 items-center overflow-x-auto">
          {tabs.map((tab) => (
            <div
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "group flex cursor-pointer items-center gap-1.5 border-r px-3 py-2 text-xs",
                tab.id === activeTab
                  ? "bg-background text-foreground"
                  : "text-muted-foreground hover:bg-muted/50"
              )}
            >
              <TabIcon type={tab.type} className="h-3 w-3" />
              <span className="max-w-24 truncate">{tab.title}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  closeTab(tab.id)
                }}
                className="ml-0.5 rounded-sm p-0.5 opacity-0 group-hover:opacity-100 hover:bg-muted"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>

        <div className="relative px-1">
          <button
            onClick={() => setMenuOpen((v) => !v)}
            className="rounded p-1.5 text-muted-foreground hover:bg-muted"
          >
            <Plus className="h-3.5 w-3.5" />
          </button>
          {menuOpen && (
            <div
              ref={menuRef}
              className="absolute top-full right-0 z-50 mt-1 w-40 rounded-lg border bg-popover p-1 shadow-md"
            >
              <button
                onClick={() => addTab("website", "Website")}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent"
              >
                <Globe className="h-3.5 w-3.5" />
                Website
              </button>
              <button
                onClick={() => addTab("terminal", "Terminal")}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs hover:bg-accent"
              >
                <Terminal className="h-3.5 w-3.5" />
                Terminal
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        <div className="relative h-full w-full">
          <div
            style={{ display: active?.type === "topology" ? "block" : "none" }}
            className="absolute inset-0"
          >
            <AgentTopology topologyState={topologyState} />
          </div>
          {tabs
            .filter((t) => t.type === "website")
            .map((tab) => (
              <div
                key={tab.id}
                style={{ display: activeTab === tab.id ? "block" : "none" }}
                className="absolute inset-0"
              >
                <CanvasWebsite initialUrl={tab.url} />
              </div>
            ))}
          {tabs
            .filter((t) => t.type === "terminal")
            .map((tab) => (
              <div
                key={tab.id}
                style={{ display: activeTab === tab.id ? "block" : "none" }}
                className="absolute inset-0"
              >
                <CanvasTerminal />
              </div>
            ))}
          {!active && (
            <div className="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground">
              Select or create a tab
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function TabIcon({ type, className }: { type: TabType; className?: string }) {
  switch (type) {
    case "topology":
      return <CircuitBoard className={className} />
    case "website":
      return <Globe className={className} />
    case "terminal":
      return <Terminal className={className} />
  }
}
