import { memo, useMemo } from "react"
import { BaseEdge, Position, type EdgeProps } from "@xyflow/react"
import { motion } from "framer-motion"

export type BeamEdgeData = {
  active?: boolean
}

type Pt = { x: number; y: number }

const CURVATURE = 0.25

function controlPoint(
  pos: Position,
  x1: number,
  y1: number,
  x2: number,
  y2: number
): Pt {
  const offset = (dist: number) => Math.abs(dist) * CURVATURE
  switch (pos) {
    case Position.Left:
      return { x: x1 - offset(x1 - x2), y: y1 }
    case Position.Right:
      return { x: x1 + offset(x2 - x1), y: y1 }
    case Position.Top:
      return { x: x1, y: y1 - offset(y1 - y2) }
    case Position.Bottom:
    default:
      return { x: x1, y: y1 + offset(y2 - y1) }
  }
}

function sampleBezier(
  p0: Pt,
  p1: Pt,
  p2: Pt,
  p3: Pt,
  count = 48
): Pt[] {
  const points: Pt[] = []
  for (let i = 0; i <= count; i++) {
    const t = i / count
    const mt = 1 - t
    const x =
      mt * mt * mt * p0.x +
      3 * mt * mt * t * p1.x +
      3 * mt * t * t * p2.x +
      t * t * t * p3.x
    const y =
      mt * mt * mt * p0.y +
      3 * mt * mt * t * p1.y +
      3 * mt * t * t * p2.y +
      t * t * t * p3.y
    points.push({ x, y })
  }
  return points
}

function BeamEdgeComponent({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition = Position.Bottom,
  targetPosition = Position.Top,
  markerEnd,
  data,
}: EdgeProps) {
  const active = (data as BeamEdgeData | undefined)?.active ?? false

  const curve = useMemo(() => {
    const sc = controlPoint(
      sourcePosition,
      sourceX,
      sourceY,
      targetX,
      targetY
    )
    const tc = controlPoint(
      targetPosition,
      targetX,
      targetY,
      sourceX,
      sourceY
    )
    const path = `M${sourceX},${sourceY} C${sc.x},${sc.y} ${tc.x},${tc.y} ${targetX},${targetY}`
    const points = sampleBezier(
      { x: sourceX, y: sourceY },
      sc,
      tc,
      { x: targetX, y: targetY }
    )
    return { path, points }
  }, [sourcePosition, targetPosition, sourceX, sourceY, targetX, targetY])

  const times = useMemo(
    () => curve.points.map((_, i) => i / (curve.points.length - 1)),
    [curve.points]
  )

  const gradientId = `beam-gradient-${id}`

  return (
    <g>
      <defs>
        <linearGradient
          id={gradientId}
          gradientUnits="userSpaceOnUse"
          x1={sourceX}
          y1={sourceY}
          x2={targetX}
          y2={targetY}
        >
          <stop offset="0%" stopColor="#3b82f6" />
          <stop offset="100%" stopColor="#22d3ee" />
        </linearGradient>
      </defs>

      <BaseEdge
        id={id}
        path={curve.path}
        markerEnd={markerEnd}
        style={{
          stroke: active ? "rgba(34, 211, 238, 0.15)" : "#94a3b8",
          strokeWidth: active ? 4 : 1.5,
          strokeOpacity: active ? 1 : 0.5,
        }}
      />

      {active && (
        <>
          <motion.path
            d={curve.path}
            fill="none"
            stroke={`url(#${gradientId})`}
            strokeWidth={2}
            strokeLinecap="round"
            strokeDasharray="7 11"
            initial={{ strokeDashoffset: 0 }}
            animate={{ strokeDashoffset: -36 }}
            transition={{
              duration: 1.1,
              repeat: Infinity,
              ease: "linear",
            }}
            style={{ pointerEvents: "none" }}
          />

          <motion.circle
            r={3}
            fill="#67e8f9"
            style={{ pointerEvents: "none" }}
            initial={false}
            animate={{
              cx: curve.points.map((p) => p.x),
              cy: curve.points.map((p) => p.y),
              opacity: curve.points.map((_, i) =>
                i === 0 || i === curve.points.length - 1 ? 0 : 1
              ),
            }}
            transition={{
              duration: 1.7,
              repeat: Infinity,
              ease: "linear",
              times,
            }}
          />

          <motion.circle
            r={7}
            fill="#22d3ee"
            opacity={0.25}
            style={{ pointerEvents: "none" }}
            initial={false}
            animate={{
              cx: curve.points.map((p) => p.x),
              cy: curve.points.map((p) => p.y),
            }}
            transition={{
              duration: 1.7,
              repeat: Infinity,
              ease: "linear",
              times,
            }}
          />
        </>
      )}
    </g>
  )
}

export const BeamEdge = memo(BeamEdgeComponent)
