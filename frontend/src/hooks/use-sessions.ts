import { useState, useCallback, useEffect, useRef } from "react"
import {
  GetSessions,
  CreateSession,
  DeleteSession,
  RenameSession,
  GetSessionMessages,
  GetWorkspace,
} from "../../wailsjs/go/main/App"
import type { Message as DBMessage } from "@/hooks/use-chat-state"

type RawSession = {
  id: string
  title: string
  workspace: string
  created_at: string
  updated_at: string
}

type RawMessage = {
  id: string
  session_id: string
  role: string
  content: string
  thinking: string
  tool_calls: string
  timestamp: string
}

export type SessionInfo = {
  id: string
  title: string
  workspace: string
  createdAt: string
  updatedAt: string
}

function formatRawSession(s: RawSession): SessionInfo {
  return {
    id: s.id,
    title: s.title,
    workspace: s.workspace,
    createdAt: s.created_at,
    updatedAt: s.updated_at,
  }
}

function parseToolCalls(json: string): Record<string, unknown>[] {
  if (!json) return []
  try {
    const parsed = JSON.parse(json)
    return Array.isArray(parsed) ? parsed.filter(Boolean) : []
  } catch {
    return []
  }
}

export function rawToMessage(m: RawMessage): DBMessage {
  return {
    id: m.id,
    role: m.role as "user" | "assistant",
    content: m.content || "",
    thinking: m.thinking || "",
    toolCalls: parseToolCalls(m.tool_calls).map((tc) => ({
      id: (tc.id as string) ?? `tc-${Date.now()}-${Math.random()}`,
      name: (tc.name as string) ?? "tool",
      args: (tc.args as Record<string, unknown>) || {},
      status: "completed" as const,
      output: tc.output as string | undefined,
    })),
    timestamp: new Date(m.timestamp),
  }
}

export function useSessions(workspace: string | null) {
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null)
  const [sessionMessages, setSessionMessages] = useState<DBMessage[]>([])
  const loadingRef = useRef(false)

  const initialLoadRef = useRef(true)

  const refreshSessions = useCallback(async () => {
    const ws = workspace ?? (await GetWorkspace())
    const raw = await GetSessions(ws)
    const list = (raw || []).map(formatRawSession)
    setSessions(list)
    if (initialLoadRef.current && list.length > 0) {
      initialLoadRef.current = false
      setCurrentSessionId(list[0]!.id)
      const msgsRaw = await GetSessionMessages(list[0]!.id)
      setSessionMessages((msgsRaw || []).map(rawToMessage))
    }
  }, [workspace])

  const createSession = useCallback(async (): Promise<string | null> => {
    const ws = workspace ?? (await GetWorkspace())
    const s = await CreateSession(ws)
    if (!s.id) return null
    await refreshSessions()
    setCurrentSessionId(s.id)
    setSessionMessages([])
    return s.id
  }, [workspace, refreshSessions])

  const selectSession = useCallback(async (id: string) => {
    setCurrentSessionId(id)
    const raw = await GetSessionMessages(id)
    const msgs = (raw || []).map(rawToMessage)
    setSessionMessages(msgs)
  }, [])

  const deleteSession = useCallback(
    async (id: string) => {
      await DeleteSession(id)
      await refreshSessions()
      if (currentSessionId === id) {
        setCurrentSessionId(null)
        setSessionMessages([])
      }
    },
    [refreshSessions, currentSessionId]
  )

  const renameSession = useCallback(
    async (id: string, title: string) => {
      await RenameSession(id, title)
      await refreshSessions()
    },
    [refreshSessions]
  )

  const refreshSessionMessages = useCallback(async () => {
    if (!currentSessionId) return
    const raw = await GetSessionMessages(currentSessionId)
    const msgs = (raw || []).map(rawToMessage)
    setSessionMessages(msgs)
  }, [currentSessionId])

  const ensureSession = useCallback(async (): Promise<string | null> => {
    if (currentSessionId) return currentSessionId
    if (loadingRef.current) return null
    loadingRef.current = true
    try {
      const ws = workspace ?? (await GetWorkspace())
      const raw = await GetSessions(ws)
      const list = (raw || []).map(formatRawSession)
      setSessions(list)
      if (list.length > 0) {
        setCurrentSessionId(list[0]!.id)
        const msgsRaw = await GetSessionMessages(list[0]!.id)
        setSessionMessages((msgsRaw || []).map(rawToMessage))
        return list[0]!.id
      }
      return await createSession()
    } finally {
      loadingRef.current = false
    }
  }, [workspace, currentSessionId, createSession])

  useEffect(() => {
    refreshSessions()
  }, [refreshSessions])

  return {
    sessions,
    currentSessionId,
    sessionMessages,
    setSessionMessages,
    createSession,
    selectSession,
    deleteSession,
    renameSession,
    refreshSessions,
    refreshSessionMessages,
    ensureSession,
  }
}
