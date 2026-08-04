import {
  useChatState,
  type Message as ChatMessage,
} from "@/hooks/use-chat-state"
import { useSessions } from "@/hooks/use-sessions"
import { useWorkspace } from "@/hooks/use-workspace"
import { ChatInterface } from "@/components/chat-interface"
import { PromptSection } from "@/components/prompt-section"
import { useModel } from "@/hooks/use-model"
import { SessionSidebar } from "@/components/session-sidebar"
import { WorkspaceDialog } from "@/components/workspace-dialog"
import { CanvasPanel } from "@/components/canvas-panel"
import { PermissionDialog } from "@/components/permission-dialog"
import { HumanInputDialog } from "@/components/human-input-dialog"
import { ProviderSettings } from "@/components/providers-settings"
import {
  SidebarProvider,
  SidebarInset,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import {
  ChatContainerRoot,
  ChatContainerContent,
} from "@/components/ui/chat-container"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { Trash2, FolderOpen, PanelRightOpen } from "lucide-react"
import { useState, useEffect, useCallback, useRef } from "react"

const WELCOME_MESSAGE: ChatMessage = {
  id: "welcome",
  role: "assistant",
  content: "Hello! I'm your AI coding agent. How can I help you?",
  thinking: "",
  toolCalls: [],
  timestamp: new Date(),
}

export function AgentChat() {
const {
    messages,
    isLoading,
    pendingPermission,
    pendingHumanInput,
    browserState,
    topologyState,
    sendMessage,
    clearMessages,
    setMessages,
    stopGeneration,
    respondPermission,
    respondHumanInput,
  } = useChatState()
  const { workspace, setWorkspace } = useWorkspace()
  const { models, currentModel, selectModel, refreshModels } = useModel()
  const {
    sessions,
    currentSessionId,
    sessionMessages,
    createSession,
    selectSession,
    deleteSession,
    ensureSession,
    refreshSessions,
    refreshSessionMessages,
  } = useSessions(workspace)
  const [showWorkspace, setShowWorkspace] = useState(false)
  const [showProviders, setShowProviders] = useState(false)
  const [canvasOpen, setCanvasOpen] = useState(false)
  const [chatRatio, setChatRatio] = useState(0.35)
  const [isDragging, setIsDragging] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const handleSend = useCallback(
    async (content: string) => {
      const sid = await ensureSession()
      if (!sid) return sendMessage(content)
      await sendMessage(content, sid)
      refreshSessions()
      refreshSessionMessages()
    },
    [ensureSession, sendMessage, refreshSessions, refreshSessionMessages]
  )

  const handleNewChat = useCallback(async () => {
    await createSession()
  }, [createSession])

  const handleSelectSession = useCallback(
    async (id: string) => {
      await selectSession(id)
    },
    [selectSession]
  )

  const handleDeleteSession = useCallback(
    async (id: string) => {
      if (currentSessionId === id) {
        clearMessages()
      }
      await deleteSession(id)
    },
    [currentSessionId, clearMessages, deleteSession]
  )

  const handleWorkspaceSelect = useCallback(
    async (path: string) => {
      await setWorkspace(path)
      setShowWorkspace(false)
    },
    [setWorkspace]
  )

  const startDrag = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }, [])

  useEffect(() => {
    if (!isDragging) return
    const container = containerRef.current
    if (!container) return

    const onMouseMove = (e: MouseEvent) => {
      const rect = container.getBoundingClientRect()
      const x = e.clientX - rect.left
      const ratio = Math.min(Math.max(x / rect.width, 0.25), 0.6)
      setChatRatio(ratio)
    }

    const onMouseUp = () => {
      setIsDragging(false)
    }

    document.addEventListener("mousemove", onMouseMove)
    document.addEventListener("mouseup", onMouseUp)
    return () => {
      document.removeEventListener("mousemove", onMouseMove)
      document.removeEventListener("mouseup", onMouseUp)
    }
  }, [isDragging])

  useEffect(() => {
    if (!currentSessionId) return
    setMessages(
      sessionMessages.length > 0 ? sessionMessages : [WELCOME_MESSAGE]
    )
  }, [sessionMessages, currentSessionId, setMessages])

  return (
    <SidebarProvider>
      <SessionSidebar
        sessions={sessions}
        currentSessionId={currentSessionId}
        onSelectSession={handleSelectSession}
        onCreateSession={handleNewChat}
        onDeleteSession={handleDeleteSession}
        onOpenWorkspace={() => setShowWorkspace(true)}
        onOpenProviders={() => setShowProviders(true)}
        workspace={workspace}
      />

      <PermissionDialog
        request={pendingPermission}
        onAllow={() => respondPermission(true)}
        onDeny={() => respondPermission(false)}
      />

      <HumanInputDialog
        request={pendingHumanInput}
        onSubmit={respondHumanInput}
        onCancel={(id) => respondHumanInput(id, "")}
      />

      <div
        ref={containerRef}
        className={cn(
          "relative flex flex-1 overflow-hidden",
          isDragging && "select-none"
        )}
      >
        <SidebarInset
          className={cn(
            "flex h-dvh flex-col",
            !isDragging && "transition-[width] duration-500 ease-in-out"
          )}
          style={{ width: canvasOpen ? `${chatRatio * 100}%` : "100%" }}
        >
          {showWorkspace && (
            <WorkspaceDialog onSelect={handleWorkspaceSelect} />
          )}

          <ProviderSettings
            open={showProviders}
            onClose={() => {
              setShowProviders(false)
              refreshModels()
            }}
          />

          <header className="flex shrink-0 items-center justify-between border-b px-4 py-2">
            <div className="flex items-center gap-2">
              <SidebarTrigger />
              <h1 className="text-sm font-medium">Demios</h1>
            </div>
            <div className="flex items-center gap-1">
              {workspace ? (
                <>
                  <button
                    onClick={() => setShowWorkspace(true)}
                    className="flex max-w-40 items-center gap-1.5 truncate rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-muted"
                    title={workspace}
                  >
                    <FolderOpen className="h-3 w-3 shrink-0" />
                    <span className="truncate">{workspace}</span>
                  </button>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    onClick={clearMessages}
                    title="Clear chat"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => setCanvasOpen((v) => !v)}
                    title="Toggle canvas"
                    data-active={canvasOpen}
                    className={cn(canvasOpen && "bg-muted text-foreground")}
                  >
                    <PanelRightOpen className="h-3.5 w-3.5" />
                  </Button>
                </>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowWorkspace(true)}
                >
                  <FolderOpen className="h-3 w-3" />
                  Set workspace
                </Button>
              )}
            </div>
          </header>

          <ChatContainerRoot className="flex-1">
            <ChatContainerContent className="flex-1">
              <ChatInterface messages={messages} isLoading={isLoading} />
            </ChatContainerContent>
          </ChatContainerRoot>

          <PromptSection
            models={models}
            currentModel={currentModel}
            onSelectModel={selectModel}
            onSubmit={handleSend}
            browserActive={browserState.active}
            isLoading={isLoading}
            onStop={stopGeneration}
            workspaceMissing={!workspace}
            onOpenProviders={() => setShowProviders(true)}
          />
        </SidebarInset>

        <div
          className={cn(
            "overflow-hidden",
            !isDragging &&
              "transition-[width,opacity] duration-500 ease-in-out",
            canvasOpen && "border-l border-border"
          )}
          style={{
            width: canvasOpen ? `${(1 - chatRatio) * 100}%` : "0px",
            opacity: canvasOpen ? 1 : 0,
          }}
        >
          <CanvasPanel topologyState={topologyState} />
        </div>

        {canvasOpen && (
          <div
            className="absolute inset-y-0 z-10 w-1.5 cursor-col-resize hover:bg-accent/50"
            style={{ left: `${chatRatio * 100}%` }}
            onMouseDown={startDrag}
          />
        )}
      </div>
    </SidebarProvider>
  )
}
