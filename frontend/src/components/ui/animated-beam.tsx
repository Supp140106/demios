import { motion } from "framer-motion"
import { useId } from "react"
import { cn } from "@/lib/utils"

interface AnimatedBeamProps {
  fromX: number
  fromY: number
  toX: number
  toY: number
  curvature?: number
  active?: boolean
  className?: string
  gradientStart?: string
  gradientEnd?: string
  delay?: number
  duration?: number
}

const pointOnQuad = (t: number, p0: number, p1: number, p2: number): number =>
  (1 - t) * (1 - t) * p0 + 2 * (1 - t) * t * p1 + t * t * p2

function quadraticPoints(
  fromX: number,
  fromY: number,
  midX: number,
  midY: number,
  toX: number,
  toY: number,
  steps = 100
): { x: number; y: number }[] {
  const pts = []
  for (let i = 0; i <= steps; i++) {
    const t = i / steps
    pts.push({
      x: pointOnQuad(t, fromX, midX, toX),
      y: pointOnQuad(t, fromY, midY, toY),
    })
  }
  return pts
}

export function AnimatedBeam({
  fromX,
  fromY,
  toX,
  toY,
  curvature = 0,
  active = true,
  className,
  gradientStart = "#06b6d4",
  gradientEnd = "#ec4899",
  delay = 0,
  duration = 2,
}: AnimatedBeamProps) {
  const id = useId()

  const midX = (fromX + toX) / 2 + (curvature || (toY - fromY) * 0.2)
  const midY = (fromY + toY) / 2

  const d = `M ${fromX} ${fromY} Q ${midX} ${midY} ${toX} ${toY}`

  const pts = quadraticPoints(fromX, fromY, midX, midY, toX, toY, 60)

  return (
    <svg
      className={cn(
        "pointer-events-none absolute top-0 left-0 h-full w-full",
        className
      )}
      style={{ overflow: "visible" }}
    >
      <defs>
        <linearGradient
          id={`beam-gradient-${id}`}
          x1="0%"
          y1="0%"
          x2="100%"
          y2="0%"
        >
          <stop offset="0%" stopColor={gradientStart} />
          <stop offset="100%" stopColor={gradientEnd} />
        </linearGradient>
      </defs>
      <motion.path
        d={d}
        fill="none"
        stroke={`url(#beam-gradient-${id})`}
        strokeWidth="1.5"
        strokeLinecap="round"
        initial={{ pathLength: 0, opacity: 0 }}
        animate={
          active
            ? {
                pathLength: [0, 1],
                opacity: [0.3, 1, 0.3],
              }
            : { pathLength: 0, opacity: 0 }
        }
        transition={{
          duration,
          delay,
          repeat: active ? Infinity : 0,
          repeatDelay: 1,
          ease: "easeInOut",
        }}
      />
      <motion.circle
        r="2.5"
        fill={gradientEnd}
        style={{ filter: `drop-shadow(0 0 4px ${gradientEnd})` }}
        initial={{ opacity: 0 }}
        animate={
          active
            ? {
                cx: pts.map((p) => p.x),
                cy: pts.map((p) => p.y),
                opacity: [0, 1, 1, 0],
              }
            : { opacity: 0 }
        }
        transition={{
          duration,
          delay,
          repeat: active ? Infinity : 0,
          repeatDelay: 1,
          ease: "linear",
        }}
      />
      <motion.circle
        r="2"
        fill={gradientStart}
        initial={{ opacity: 0 }}
        animate={
          active
            ? {
                cx: pts
                  .slice()
                  .reverse()
                  .map((p) => p.x),
                cy: pts
                  .slice()
                  .reverse()
                  .map((p) => p.y),
                opacity: [1, 0],
              }
            : { opacity: 0 }
        }
        transition={{
          duration: duration * 0.8,
          delay,
          repeat: active ? Infinity : 0,
          repeatDelay: 1.2,
          ease: "linear",
        }}
      />
    </svg>
  )
}
