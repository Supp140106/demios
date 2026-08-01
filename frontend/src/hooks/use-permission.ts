import { useState, useCallback, useRef } from "react"

export type PermissionRequest = {
  id: string
  name: string
  args: Record<string, unknown>
}

export function usePermission() {
  const [pending, setPending] = useState<PermissionRequest | null>(null)
  const resolveRef = useRef<((allowed: boolean) => void) | null>(null)

  const requestPermission = useCallback(
    (req: PermissionRequest): Promise<boolean> => {
      return new Promise((resolve) => {
        resolveRef.current = resolve
        setPending(req)
      })
    },
    []
  )

  const respond = useCallback((allowed: boolean) => {
    resolveRef.current?.(allowed)
    resolveRef.current = null
    setPending(null)
  }, [])

  const cancel = useCallback(() => {
    respond(false)
  }, [respond])

  return { pending, requestPermission, respond, cancel }
}
