import { useCallback, useEffect, useMemo, useRef } from "react"
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type OnConnect,
  type NodeTypes,
  type EdgeTypes,
} from "@xyflow/react"
import { AgentNode } from "@/components/agent-node"
import { BeamEdge } from "@/components/beam-edge"
import type { TopologyState } from "@/hooks/use-chat-state"
import "@xyflow/react/dist/style.css"
import { cn } from "@/lib/utils"

const nodeTypes: NodeTypes = {
  agent: AgentNode,
}

const edgeTypes: EdgeTypes = {
  beam: BeamEdge,
}

const initialNodes: Node[] = [
  {
    id: "main-agent",
    type: "agent",
    position: { x: 250, y: 20 },
    data: {
      label: "Main Agent",
      status: "idle",
      action: "Ready",
    },
  },
  {
    id: "browser-agent",
    type: "agent",
    position: { x: 250, y: 200 },
    data: {
      label: "Browser Agent",
      status: "idle",
      action: "Idle",
    },
  },
  {
    id: "dev-server",
    type: "agent",
    position: { x: 250, y: 380 },
    data: {
      label: "Dev Server",
      status: "idle",
      action: "Not running",
    },
  },
]

const initialEdges: Edge[] = [
  {
    id: "e-main-browser",
    source: "main-agent",
    target: "browser-agent",
    sourceHandle: null,
    targetHandle: null,
    type: "beam",
    data: { active: false },
  },
  {
    id: "e-browser-server",
    source: "browser-agent",
    target: "dev-server",
    sourceHandle: null,
    targetHandle: null,
    type: "beam",
    data: { active: false },
  },
]

interface AgentTopologyProps {
  topologyState?: TopologyState
}

export function AgentTopology({ topologyState }: AgentTopologyProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)
  const reactFlowWrapper = useRef<HTMLDivElement>(null)
  const positionsRef = useRef<Record<string, { x: number; y: number }>>({})

  const liveNodes = useMemo(
    () => topologyState?.nodes ?? [],
    [topologyState?.nodes]
  )
  const liveEdges = useMemo(
    () => topologyState?.edges ?? [],
    [topologyState?.edges]
  )

  useEffect(() => {
    setNodes((current) =>
      current.map((node) => {
        const live = liveNodes.find((n) => n.id === node.id)
        if (!live) return node
        positionsRef.current[node.id] = node.position
        return {
          ...node,
          position: positionsRef.current[node.id] ?? node.position,
          data: { ...node.data, ...live },
        }
      })
    )
  }, [liveNodes, setNodes])

  useEffect(() => {
    setEdges((current) =>
      current.map((edge) => {
        const live = liveEdges.find((e) => e.id === edge.id)
        if (!live) return edge
        return {
          ...edge,
          type: "beam",
          data: { active: Boolean(live.active) },
        }
      })
    )
  }, [liveEdges, setEdges])

  const onConnect: OnConnect = useCallback(
    (connection) => {
      setEdges((eds) =>
        eds.concat({
          ...connection,
          id: `e-${connection.source}-${connection.target}`,
          type: "beam",
          data: { active: false },
        })
      )
    },
    [setEdges]
  )

  return (
    <div className={cn("relative h-full w-full")} ref={reactFlowWrapper}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        attributionPosition="bottom-left"
        className="bg-background"
      >
        <Background />
        <Controls />
        <MiniMap
          nodeColor={(n) => {
            const status = n.data?.status as string
            if (status === "running" || status === "analyzing") return "#3b82f6"
            if (status === "done") return "#22c55e"
            if (status === "error") return "#ef4444"
            return "#64748b"
          }}
          pannable
          zoomable
        />
      </ReactFlow>
    </div>
  )
}
