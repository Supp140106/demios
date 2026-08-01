import { useState, useCallback, useRef } from "react"
import { GetServerPort } from "../../wailsjs/go/main/App"

export type ToolCallState = {
  id: string
  name: string
  args: Record<string, unknown>
  status: "pending" | "running" | "completed" | "error"
  output?: string
  error?: string
  diff?: string
  diffs?: { path: string; patch: string }[]
}

export type Message = {
  id: string
  role: "user" | "assistant"
  content: string
  thinking: string
  toolCalls: ToolCallState[]
  timestamp: Date
}

export type PermissionRequest = {
  id: string
  name: string
  args: Record<string, unknown>
}

export type HumanInputRequest = {
  id: string
  question: string
  options?: string[]
}

export type TopologyNode = {
  id: string
  label: string
  status: "idle" | "connected" | "running" | "analyzing" | "done" | "error"
  action?: string
  message?: string
  screenshot?: string
}

export type TopologyEdge = {
  id: string
  source: string
  target: string
  active?: boolean
}

export type TopologyState = {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
  active: boolean
}

type ServerMeta = {
  id?: string
  port?: number
  url?: string
  status?: string
  project_dir?: string
}

export type BrowserState = {
  active: boolean
  url: string
  title: string
  screenshot: string | null
  mode: "agent" | "user" | null
}

let cachedPort: string | null = null

async function getPort(): Promise<string> {
  if (!cachedPort) cachedPort = await GetServerPort()
  return cachedPort
}

let messageCounter = 0

function createMessage(role: "user" | "assistant", content = ""): Message {
  messageCounter++
  return {
    id: `msg-${messageCounter}-${Date.now()}`,
    role,
    content,
    thinking: "",
    toolCalls: [],
    timestamp: new Date(),
  }
}

export function useChatState() {
  const [messages, setMessages] = useState<Message[]>([
    createMessage(
      "assistant",
      "Hello! I'm your AI coding agent. How can I help you?"
    ),
  ])
  const [isLoading, setIsLoading] = useState(false)
  const [pendingPermission, setPendingPermission] =
    useState<PermissionRequest | null>(null)
  const [pendingHumanInput, setPendingHumanInput] =
    useState<HumanInputRequest | null>(null)
  const [browserState, setBrowserState] = useState<BrowserState>({
    active: false,
    url: "",
    title: "",
    screenshot: null,
    mode: null,
  })
  const [topologyState, setTopologyState] = useState<TopologyState>({
    nodes: [
      { id: "main-agent", label: "Main Agent", status: "idle", action: "Ready" },
      { id: "browser-agent", label: "Browser Agent", status: "idle", action: "Idle" },
      { id: "dev-server", label: "Dev Server", status: "idle", action: "Not running" },
    ],
    edges: [
      { id: "e-main-browser", source: "main-agent", target: "browser-agent" },
      { id: "e-browser-server", source: "browser-agent", target: "dev-server" },
    ],
    active: false,
  })
  const abortRef = useRef<AbortController | null>(null)

  const respondPermission = useCallback(
    async (allowed: boolean) => {
      const req = pendingPermission
      if (!req) return
      setPendingPermission(null)
      try {
        const port = await getPort()
        await fetch(`http://${port}/api/permission/respond`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id: req.id, allowed }),
        })
      } catch {
        // ignore
      }
    },
    [pendingPermission]
  )

   const respondHumanInput = useCallback(async (id: string, input: string) => {
     setPendingHumanInput(null)
     try {
       const port = await getPort()
       await fetch(`http://${port}/api/human-input/respond`, {
         method: "POST",
         headers: { "Content-Type": "application/json" },
         body: JSON.stringify({ id, input }),
       })
     } catch {
       // ignore
     }
   }, [])

   const updateTopologyNode = useCallback(
     (nodeId: string, updates: Partial<TopologyNode>) => {
       setTopologyState((prev) => ({
         ...prev,
         nodes: prev.nodes.map((n) =>
           n.id === nodeId ? { ...n, ...updates } : n
         ),
       }))
     },
     []
   )

   const updateTopologyEdge = useCallback(
     (edgeId: string, updates: Partial<TopologyEdge>) => {
       setTopologyState((prev) => ({
         ...prev,
         edges: prev.edges.map((e) =>
           e.id === edgeId ? { ...e, ...updates } : e
         ),
       }))
     },
     []
   )

   const setTopologyActive = useCallback((active: boolean) => {
     setTopologyState((prev) => ({
       ...prev,
       active,
       edges: prev.edges.map((e) => ({ ...e, active })),
     }))
   }, [])

   const sendMessage = useCallback(
    async (content: string, sessionId?: string) => {
      const userMessage = createMessage("user", content)
      setMessages((prev) => [...prev, userMessage])
      setIsLoading(true)
      updateTopologyNode("main-agent", { status: "running", action: "Working…" })
      setTopologyActive(true)

      const abortController = new AbortController()
      abortRef.current = abortController

      try {
        const port = await getPort()
        const body: Record<string, unknown> = { message: content }
        if (sessionId) body.session_id = sessionId

        const res = await fetch(`http://${port}/api/chat/stream`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
          signal: abortController.signal,
        })

        const reader = res.body!.getReader()
        const decoder = new TextDecoder()
        let buffer = ""
        let currentEvent = ""

        let assistantMsg: Message | null = null

        function ensureAssistant(): Message {
          if (!assistantMsg) {
            assistantMsg = createMessage("assistant")
            setMessages((prev) => [...prev, assistantMsg!])
          }
          return assistantMsg
        }

        function updateAssist(fn: (msg: Message) => Message) {
          if (!assistantMsg) return
          const msg = fn(assistantMsg)
          assistantMsg = msg
          const msgId = msg.id
          setMessages((prev) => {
            const idx = prev.findIndex((m) => m.id === msgId)
            if (idx === -1) return [...prev, msg]
            const next = [...prev]
            next[idx] = msg
            return next
          })
        }

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split("\n")
          buffer = lines.pop()!

          for (const line of lines) {
            if (line.startsWith("event: ")) {
              currentEvent = line.slice(7).trim()
              continue
            }
            if (!line.startsWith("data: ")) continue
            const raw = line.slice(6)
            if (raw === "[DONE]") {
              updateAssist((m) => {
                if (!m.content && !m.thinking) {
                  return { ...m, content: "Task completed." }
                }
                return m
              })
              break
            }

            try {
              const parsed = JSON.parse(raw)

              switch (currentEvent) {
                case "think": {
                  ensureAssistant()
                  updateAssist((m) => ({
                    ...m,
                    thinking: m.thinking + (parsed.token || ""),
                  }))
                  updateTopologyNode("main-agent", {
                    status: "analyzing",
                    action: "Thinking…",
                  })
                  break
                }
                case "tool-call": {
                  ensureAssistant()
                  const tc: ToolCallState = {
                    id: parsed.id,
                    name: parsed.name,
                    args: parsed.args || {},
                    status: "running",
                  }
                  updateAssist((m) => ({
                    ...m,
                    toolCalls: [...m.toolCalls, tc],
                  }))
                  const tcName = parsed.name as string
                  if (tcName === "StartServer") {
                    updateTopologyNode("dev-server", {
                      status: "running",
                      action: "Starting server…",
                    })
                  } else if (tcName === "StopServer") {
                    updateTopologyNode("dev-server", {
                      status: "idle",
                      action: "Stopping…",
                    })
                  } else if (tcName === "TestWebsite") {
                    updateTopologyNode("main-agent", {
                      status: "running",
                      action: "Delegating to Browser Agent…",
                    })
                    updateTopologyNode("browser-agent", {
                      status: "connected",
                      action: "Standing by",
                    })
                    updateTopologyEdge("e-main-browser", { active: true })
                  }
                  break
                }
                case "tool-result": {
                  updateAssist((m) => ({
                    ...m,
                    toolCalls: m.toolCalls.map((t) => {
                      if (t.id === parsed.id) {
                        const out: ToolCallState = {
                          id: parsed.id,
                          name: parsed.name,
                          args: parsed.args || t.args,
                          status: parsed.error ? "error" : "completed",
                          output: parsed.output,
                          error: parsed.error,
                        }
                        if (parsed.diff) out.diff = parsed.diff
                        if (parsed.diffs) out.diffs = parsed.diffs
                        return out
                      }
                      return t
                    }),
                  }))
                  const trName = parsed.name as string
                  if (trName === "StartServer" && !parsed.error) {
                    const server = parsed.server as ServerMeta | undefined
                    const port = server?.port
                    const url = server?.url
                    updateTopologyNode("dev-server", {
                      status: "done",
                      action: url
                        ? `Running at ${url}`
                        : port
                          ? `Running on :${port}`
                          : "Running",
                      message: server?.project_dir
                        ? server.project_dir
                        : undefined,
                    })
                  } else if (trName === "StopServer" && !parsed.error) {
                    updateTopologyNode("dev-server", {
                      status: "idle",
                      action: "Not running",
                      message: undefined,
                    })
                  } else if (trName === "TestWebsite" && !parsed.error) {
                    updateTopologyEdge("e-main-browser", { active: false })
                  }
                  break
                }
                case "token": {
                  ensureAssistant()
                  updateAssist((m) => ({
                    ...m,
                    content: m.content + (parsed.token || ""),
                  }))
                  break
                }
                case "iteration": {
                  if (assistantMsg) {
                    assistantMsg = null
                  }
                  updateTopologyNode("main-agent", {
                    status: "analyzing",
                    action: "Working…",
                  })
                  break
                }
                case "permission-request": {
                  setPendingPermission({
                    id: parsed.id,
                    name: parsed.name,
                    args: parsed.args || {},
                  })
                  break
                }
                case "human-input-request": {
                  setPendingHumanInput({
                    id: parsed.id,
                    question: parsed.question,
                    options: parsed.options,
                  })
                  break
                }
                 case "error": {
                   ensureAssistant()
                   updateAssist((m) => ({
                     ...m,
                     content: m.content || `Error: ${parsed.error}`,
                   }))
                   updateTopologyNode("main-agent", {
                     status: "error",
                     action: "Error occurred",
                   })
                   updateTopologyEdge("e-main-browser", { active: false })
                   updateTopologyEdge("e-browser-server", { active: false })
                   break
                 }
                 case "browser-open":
                 case "browser-opened": {
                   setBrowserState((prev) => ({
                     ...prev,
                     active: true,
                     url: parsed.url || prev.url,
                     title: parsed.title || prev.title,
                   }))
                   updateTopologyNode("browser-agent", {
                     status: "connected",
                     action: "Browser opened",
                   })
                   break
                 }
                 case "page-navigated": {
                   setBrowserState((prev) => ({
                     ...prev,
                     url: parsed.url || prev.url,
                     title: parsed.title || prev.title,
                   }))
                   updateTopologyNode("browser-agent", {
                     status: "running",
                     action: `Navigated to ${parsed.url || "page"}`,
                   })
                   updateTopologyEdge("e-browser-server", { active: true })
                   break
                 }
                 case "browser-action": {
                   ensureAssistant()
                   updateAssist((m) => ({
                     ...m,
                     content: m.content + (parsed.status || ""),
                   }))
                   break
                 }
                 case "browser-screenshot": {
                   setBrowserState((prev) => ({
                     ...prev,
                     screenshot: parsed.screenshot || prev.screenshot,
                   }))
                   updateTopologyNode("browser-agent", {
                     status: "analyzing",
                     action: "Analyzing page",
                     screenshot: parsed.screenshot || undefined,
                   })
                   break
                 }
                 case "browser-control": {
                   setBrowserState((prev) => ({
                     ...prev,
                     mode: parsed.mode || prev.mode,
                   }))
                   break
                 }
                 case "browser-stop":
                 case "browser-stopped": {
                   ensureAssistant()
                   updateAssist((m) => ({
                     ...m,
                     content: m.content || "Browser session stopped.",
                   }))
                   setBrowserState({
                     active: false,
                     url: "",
                     title: "",
                     screenshot: null,
                     mode: null,
                   })
                   updateTopologyNode("browser-agent", {
                     status: "idle",
                     action: "Idle",
                     screenshot: undefined,
                   })
                   updateTopologyEdge("e-main-browser", { active: false })
                   updateTopologyEdge("e-browser-server", { active: false })
                   break
                 }
                 case "browser-error": {
                   ensureAssistant()
                   updateAssist((m) => ({
                     ...m,
                     content: m.content || `Browser error: ${parsed.error}`,
                   }))
                   setBrowserState((prev) => ({ ...prev, active: false }))
                   updateTopologyNode("browser-agent", {
                     status: "error",
                     action: "Error occurred",
                   })
                   updateTopologyEdge("e-main-browser", { active: false })
                   updateTopologyEdge("e-browser-server", { active: false })
                   break
                 }
                 case "browser-done": {
                   ensureAssistant()
                   updateAssist((m) => ({
                     ...m,
                     content: m.content || "Browser task completed.",
                   }))
                   setBrowserState((prev) => ({
                     ...prev,
                     mode: "agent",
                   }))
                   updateTopologyNode("browser-agent", {
                     status: "done",
                     action: "Task complete",
                   })
                   updateTopologyEdge("e-main-browser", { active: false })
                   updateTopologyEdge("e-browser-server", { active: false })
                   break
                 }
                 case "done": {
                   ensureAssistant()
                   updateAssist((m) => {
                     if (!m.content && !m.thinking) {
                       return { ...m, content: "Task completed." }
                     }
                     return m
                   })
                   updateTopologyNode("main-agent", {
                     status: "done",
                     action: "Complete",
                   })
                     updateTopologyEdge("e-main-browser", { active: false })
                     updateTopologyEdge("e-browser-server", { active: false })
                     break
                   }
               }
             } catch {
               // skip malformed JSON
             }
          }
        }
      } catch (err) {
        if ((err as Error).name === "AbortError") return
        setMessages((prev) => [
          ...prev,
          createMessage("assistant", "Error: failed to connect to server."),
        ])
      } finally {
        setIsLoading(false)
        abortRef.current = null
      }
    },
    [setTopologyActive, updateTopologyEdge, updateTopologyNode]
  )

    const stopBrowser = useCallback(async () => {
      try {
        const port = await getPort()
        await fetch(`http://${port}/api/browser/stop`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        })
      } catch {
        // ignore
      }
    }, [])

    const takeBrowserControl = useCallback(async () => {
      try {
        const port = await getPort()
        await fetch(`http://${port}/api/browser/take-control`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        })
      } catch {
        // ignore
      }
    }, [])

    const giveBrowserControl = useCallback(async () => {
      try {
        const port = await getPort()
        await fetch(`http://${port}/api/browser/give-control`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        })
      } catch {
        // ignore
      }
    }, [])

    const stopGeneration = useCallback(() => {
      abortRef.current?.abort()
      abortRef.current = null
      setPendingPermission(null)
      setPendingHumanInput(null)
    }, [])

    const clearMessages = useCallback(() => {
      setMessages([])
    }, [])

      return {
        messages,
        isLoading,
        pendingPermission,
        pendingHumanInput,
        browserState,
        topologyState,
        sendMessage,
        stopGeneration,
        clearMessages,
        setMessages,
        respondPermission,
        respondHumanInput,
        stopBrowser,
        takeBrowserControl,
        giveBrowserControl,
        updateTopologyNode,
        updateTopologyEdge,
        setTopologyActive,
      }
}
