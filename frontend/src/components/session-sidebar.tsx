import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarFooter,
  SidebarHeader,
} from "@/components/ui/sidebar"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Plus, Trash2, MessageSquare, FolderOpen } from "lucide-react"
import { useState } from "react"
import type { SessionInfo } from "@/hooks/use-sessions"
import { cn } from "@/lib/utils"

type SessionSidebarProps = {
  sessions: SessionInfo[]
  currentSessionId: string | null
  onSelectSession: (id: string) => void
  onCreateSession: () => void
  onDeleteSession: (id: string) => void
  onOpenWorkspace: () => void
  workspace: string | null
}

const MAX_TITLE_LEN = 28

function truncateTitle(title: string): string {
  if (title.length <= MAX_TITLE_LEN) return title
  return title.slice(0, MAX_TITLE_LEN) + "…"
}

export function SessionSidebar({
  sessions,
  currentSessionId,
  onSelectSession,
  onCreateSession,
  onDeleteSession,
  onOpenWorkspace,
  workspace,
}: SessionSidebarProps) {
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  return (
    <>
      <Sidebar collapsible="icon">
        <SidebarHeader className="border-b px-3 py-2">
          <Button
            variant="outline"
            size="sm"
            className="w-full justify-start gap-2 text-xs"
            onClick={onCreateSession}
          >
            <Plus className="h-3.5 w-3.5" />
            <span className="group-data-[collapsible=icon]:hidden">
              New Chat
            </span>
          </Button>
        </SidebarHeader>

        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupLabel className="group-data-[collapsible=icon]:hidden">
              Sessions
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {sessions.length === 0 && (
                  <SidebarMenuItem>
                    <span className="px-3 py-4 text-center text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">
                      No sessions yet
                    </span>
                  </SidebarMenuItem>
                )}
                {sessions.map((session) => (
                  <SidebarMenuItem key={session.id}>
                    <SidebarMenuButton
                      isActive={session.id === currentSessionId}
                      onClick={() => onSelectSession(session.id)}
                      className="group/item flex items-center gap-2"
                    >
                      <MessageSquare className="h-3.5 w-3.5 shrink-0" />
                      <span
                        className={cn(
                          "flex-1 truncate text-left text-xs group-data-[collapsible=icon]:hidden",
                          session.id === currentSessionId && "font-medium"
                        )}
                        title={session.title}
                      >
                        {truncateTitle(session.title)}
                      </span>
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        className="-mr-1 shrink-0 opacity-0 group-hover/item:opacity-100 group-data-[collapsible=icon]:hidden"
                        onClick={(e) => {
                          e.stopPropagation()
                          setDeleteTarget(session.id)
                        }}
                      >
                        <Trash2 className="h-3 w-3 text-destructive" />
                      </Button>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>

        <SidebarFooter className="border-t p-3">
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start gap-2 text-xs text-muted-foreground group-data-[collapsible=icon]:hidden"
            onClick={onOpenWorkspace}
          >
            <FolderOpen className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">
              {workspace
                ? workspace.length > 22
                  ? "..." + workspace.slice(-22)
                  : workspace
                : "Set workspace"}
            </span>
          </Button>
        </SidebarFooter>
      </Sidebar>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete session?</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete this chat session and all its
              messages. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deleteTarget) {
                  onDeleteSession(deleteTarget)
                  setDeleteTarget(null)
                }
              }}
            >
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
