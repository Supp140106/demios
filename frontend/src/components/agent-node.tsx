import { memo } from "react"
import { Handle, Position, type NodeProps } from "@xyflow/react"
import { cn } from "@/lib/utils"
import { motion, AnimatePresence } from "framer-motion"

const statusColors: Record<string, string> = {
  idle: "bg-muted-foreground/30",
  connected: "bg-green-500",
  running: "bg-blue-500",
  analyzing: "bg-yellow-500",
  done: "bg-green-500",
  error: "bg-red-500",
}

const activeStatuses = new Set(["running", "analyzing"])
const doneStatuses = new Set(["done", "connected", "error"])

function AgentNodeComponent({ data }: NodeProps) {
  const label = (data.label as string) ?? ""
  const status = (data.status as string) ?? "idle"
  const action = data.action as string | undefined
  const message = data.message as string | undefined
  const screenshot = data.screenshot as string | undefined

  const isActive = activeStatuses.has(status)
  const isDone = doneStatuses.has(status)

  return (
    <motion.div
      className={cn(
        "relative flex w-56 flex-col gap-2 overflow-hidden rounded-xl border bg-card p-3 shadow-sm",
        isActive && "border-blue-400/40",
        isDone && "border-green-400/30"
      )}
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.3 }}
    >
      {isActive && (
        <motion.div
          className="pointer-events-none absolute inset-0 rounded-xl"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          style={{
            boxShadow:
              "0 0 24px 0 rgba(59, 130, 246, 0.35), inset 0 0 18px 0 rgba(59, 130, 246, 0.12)",
          }}
        />
      )}

      <Handle
        type="target"
        position={Position.Top}
        className="!w-2 !h-2 !bg-cyan-400 !border-2 !border-background"
      />
      <div className="relative z-10 flex items-center gap-2">
        <span className="relative flex h-2.5 w-2.5 shrink-0">
          {isActive && (
            <motion.span
              className={cn(
                "absolute inline-flex h-full w-full rounded-full",
                statusColors[status] || statusColors.idle
              )}
              initial={{ opacity: 0.6, scale: 1 }}
              animate={{ opacity: 0, scale: 2.4 }}
              transition={{ duration: 1.2, repeat: Infinity, ease: "easeOut" }}
            />
          )}
          <span
            className={cn(
              "relative inline-flex h-2.5 w-2.5 rounded-full",
              statusColors[status] || statusColors.idle
            )}
          />
        </span>
        <span className="text-sm font-medium">{label}</span>
        {isActive && (
          <motion.span
            className="ml-auto flex gap-0.5"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.15 }}
          >
            {[0, 1, 2].map((i) => (
              <motion.span
                key={i}
                className="h-1 w-1 rounded-full bg-current text-blue-400"
                animate={{ opacity: [0.2, 1, 0.2], y: [0, -2, 0] }}
                transition={{
                  duration: 0.9,
                  repeat: Infinity,
                  ease: "easeInOut",
                  delay: i * 0.15,
                }}
              />
            ))}
          </motion.span>
        )}
      </div>
      {action && (
        <AnimatePresence mode="popLayout" initial={false}>
          <motion.span
            key={action}
            className="relative z-10 truncate text-xs text-muted-foreground"
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.25 }}
          >
            {action}
          </motion.span>
        </AnimatePresence>
      )}
      {message && (
        <p className="relative z-10 line-clamp-2 text-xs text-muted-foreground">
          {message}
        </p>
      )}
      {screenshot && (
        <motion.div
          className="relative z-10 overflow-hidden rounded-lg border"
          initial={{ opacity: 0, scale: 0.96 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ duration: 0.3, ease: "easeOut" }}
        >
          <img
            src={`data:image/png;base64,${screenshot}`}
            alt="Page screenshot"
            className="h-24 w-full object-cover"
          />
          <motion.div
            className="absolute inset-0 bg-gradient-to-t from-cyan-500/10 to-transparent"
            initial={{ opacity: 0 }}
            animate={{ opacity: [0, 1, 0.4] }}
            transition={{ duration: 1, repeat: Infinity, ease: "easeInOut" }}
          />
        </motion.div>
      )}
      <Handle
        type="source"
        position={Position.Bottom}
        className="!w-2 !h-2 !bg-cyan-400 !border-2 !border-background"
      />
    </motion.div>
  )
}

export const AgentNode = memo(AgentNodeComponent)
