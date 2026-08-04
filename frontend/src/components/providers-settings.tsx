import { useState, useEffect, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuGroup, DropdownMenuGroupLabel } from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import {
  X,
  Plus,
  ChevronDown,
  ChevronRight,
  Save,
  Trash2,
  RefreshCw,
  Eye,
  EyeOff,
  Check,
} from "lucide-react"
import {
  GetProviderPresets,
  GetProviders,
  AddProvider,
  UpdateProvider,
  RemoveProvider,
} from "../../wailsjs/go/main/App"
import type { llm } from "../../wailsjs/go/models"

type ProviderPreset = llm.ProviderPreset
type ProviderConfig = llm.ProviderConfig

interface ProviderSettingsProps {
  open: boolean
  onClose: () => void
}

interface ProviderState {
  id: string
  name: string
  preset: ProviderPreset
  apiKey: string
  baseUrl: string
  completionsUrl: string
  extraFields: Record<string, string>
  expanded: boolean
  configured: boolean
  showKey: boolean
  selectedModel: string
}

function generateId(name: string): string {
  return (
    name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "") +
    "-" +
    Date.now().toString(36)
  )
}

export function ProviderSettings({ open, onClose }: ProviderSettingsProps) {
  const [providers, setProviders] = useState<ProviderState[]>([])
  const [showAddCustom, setShowAddCustom] = useState(false)
  const [customName, setCustomName] = useState("")
  const [customBaseUrl, setCustomBaseUrl] = useState("")
  const [customCompletionsUrl, setCustomCompletionsUrl] = useState("")
  const [customApiKey, setCustomApiKey] = useState("")
  const [customAuthType, setCustomAuthType] = useState("bearer")
  const [customModel, setCustomModel] = useState("")
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const loadData = useCallback(async () => {
    try {
      const [presetsData, userProvidersData] = await Promise.all([
        GetProviderPresets(),
        GetProviders(),
      ])

      const userProviderMap = new Map<string, ProviderConfig>()
      for (const up of userProvidersData) {
        userProviderMap.set(up.Name, up)
      }

      const states: ProviderState[] = presetsData.map((preset) => {
        const existing = userProviderMap.get(preset.Name)
        const extraFields: Record<string, string> = {}
        if (existing?.ExtraFields) {
          for (const [k, v] of Object.entries(existing.ExtraFields)) {
            extraFields[k] = v
          }
        }
        return {
          id: existing?.ID || generateId(preset.Name),
          name: preset.Name,
          preset,
          apiKey: existing?.APIKey || "",
          baseUrl: preset.BaseURL,
          completionsUrl: existing?.CompletionsURL || "",
          extraFields,
          expanded: false,
          configured: !!existing,
          showKey: false,
          selectedModel: existing?.Models?.[0] || preset.Model || "",
        }
      })
      setProviders(states)
    } catch (err) {
      console.error("Failed to load provider data:", err)
    }
  }, [])

  useEffect(() => {
    if (open) {
      loadData()
    }
  }, [open, loadData])

  const toggleExpand = (id: string) => {
    setProviders((prev) =>
      prev.map((p) => (p.id === id ? { ...p, expanded: !p.expanded } : p))
    )
  }

  const updateApiKey = (id: string, value: string) => {
    setProviders((prev) =>
      prev.map((p) => (p.id === id ? { ...p, apiKey: value } : p))
    )
  }

  const updateBaseUrl = (id: string, value: string) => {
    setProviders((prev) =>
      prev.map((p) => (p.id === id ? { ...p, baseUrl: value } : p))
    )
  }

  const updateCompletionsUrl = (id: string, value: string) => {
    setProviders((prev) =>
      prev.map((p) => (p.id === id ? { ...p, completionsUrl: value } : p))
    )
  }

  const updateExtraField = (id: string, key: string, value: string) => {
    setProviders((prev) =>
      prev.map((p) =>
        p.id === id
          ? { ...p, extraFields: { ...p.extraFields, [key]: value } }
          : p
      )
    )
  }

  const updateSelectedModel = (id: string, model: string) => {
    setProviders((prev) =>
      prev.map((p) => (p.id === id ? { ...p, selectedModel: model } : p))
    )
  }

  const toggleShowKey = (id: string) => {
    setProviders((prev) =>
      prev.map((p) => (p.id === id ? { ...p, showKey: !p.showKey } : p))
    )
  }

  const saveProvider = async (state: ProviderState) => {
    try {
      const headers: Record<string, string> = {}
      if (state.preset.AuthType === "api-key" && state.preset.HeaderName) {
        headers[state.preset.HeaderName] = state.apiKey
      }
      for (const [key, val] of Object.entries(state.extraFields)) {
        if (val) headers[key] = val
      }

      const cfg: ProviderConfig = {
        ID: state.id,
        Name: state.name,
        BaseURL: state.baseUrl,
        CompletionsURL: state.completionsUrl,
        APIKey: state.apiKey,
        AuthType: state.preset.AuthType,
        HeaderName: state.preset.HeaderName,
        Headers: headers,
        Models: state.preset.Models,
        ExtraFields: state.extraFields,
        AutoDetect: state.preset.AutoDetect,
        EnvVar: state.preset.EnvVar || "",
      }

      if (state.configured) {
        await UpdateProvider(cfg)
      } else {
        await AddProvider(cfg)
      }

      setProviders((prev) =>
        prev.map((p) =>
          p.id === state.id ? { ...p, configured: true } : p
        )
      )
    } catch (err) {
      console.error("Failed to save provider:", err)
    }
  }

  const removeProvider = async (state: ProviderState) => {
    try {
      await RemoveProvider(state.id)
      setProviders((prev) =>
        prev.map((p) =>
          p.id === state.id
            ? { ...p, configured: false, apiKey: "" }
            : p
        )
      )
    } catch (err) {
      console.error("Failed to remove provider:", err)
    }
  }

  const saveAll = async () => {
    setSaving(true)
    for (const state of providers) {
      if (state.apiKey || state.configured) {
        await saveProvider(state)
      }
    }
    setSaving(false)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  const addCustomProvider = async () => {
    if (!customName || !customBaseUrl) return
    const models = customModel ? [customModel] : []
    const cfg: ProviderConfig = {
      ID: generateId(customName),
      Name: customName,
      BaseURL: customBaseUrl,
      CompletionsURL: customCompletionsUrl,
      APIKey: customApiKey,
      AuthType: customAuthType,
      HeaderName: "",
      Headers: {},
      Models: models,
      ExtraFields: {},
      AutoDetect: false,
      EnvVar: "",
    }
    await AddProvider(cfg)
    setShowAddCustom(false)
    setCustomName("")
    setCustomBaseUrl("")
    setCustomCompletionsUrl("")
    setCustomApiKey("")
    setCustomAuthType("bearer")
    setCustomModel("")
    loadData()
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="fixed inset-0 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      <div className="relative z-50 flex h-[80vh] w-full max-w-2xl flex-col rounded-xl border border-border bg-card shadow-2xl">
        <div className="flex items-center justify-between border-b border-border px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-foreground">LLM Providers</h2>
            <p className="mt-0.5 text-xs text-muted-foreground">
              Configure API keys for your AI providers
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={loadData}
              title="Refresh"
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={onClose}
              title="Close"
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-3">
          {providers.map((state) => (
            <div
              key={state.id}
              className="mb-2 rounded-lg border border-border bg-background"
            >
              <button
                type="button"
                onClick={() => toggleExpand(state.id)}
                className="flex w-full items-center gap-3 px-4 py-3 text-left"
              >
                <ProviderIcon name={state.preset.Name} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-foreground">
                      {state.preset.Name}
                    </span>
                    {state.configured && (
                      <span className="rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                        Configured
                      </span>
                    )}
                  </div>
                  <p className="mt-0.5 text-xs text-muted-foreground truncate">
                    {state.preset.Description}
                  </p>
                </div>
                {state.expanded ? (
                  <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                ) : (
                  <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                )}
              </button>

              {state.expanded && (
                <div className="border-t border-border px-4 py-3 space-y-3">
                  <div className="flex items-center gap-2">
                    <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                      API Key
                    </label>
                    <div className="relative flex-1">
                      <Input
                        type={state.showKey ? "text" : "password"}
                        value={state.apiKey}
                        onChange={(e) => updateApiKey(state.id, e.target.value)}
                        placeholder={state.preset.EnvVar ? `Or set ${state.preset.EnvVar} env var` : "Enter API key"}
                        className="h-8 pr-8 text-xs"
                      />
                      <button
                        type="button"
                        onClick={() => toggleShowKey(state.id)}
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                      >
                        {state.showKey ? (
                          <EyeOff className="h-3.5 w-3.5" />
                        ) : (
                          <Eye className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </div>
                  </div>

                  {(state.preset.BaseURL === "" ||
                  state.preset.Name === "Amazon Bedrock" ||
                  state.preset.Name === "Azure OpenAI" ||
                  state.preset.Name === "Google Vertex AI") ? (
                    <div className="flex items-center gap-2">
                      <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                        Base URL
                      </label>
                      <Input
                        type="text"
                        value={state.baseUrl}
                        onChange={(e) => updateBaseUrl(state.id, e.target.value)}
                        placeholder="https://api.example.com/v1"
                        className="h-8 text-xs"
                      />
                    </div>
                  ) : null}

                  {(state.preset.Name === "Azure OpenAI") ? (
                    <div className="flex items-center gap-2">
                      <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                        Completions URL
                      </label>
                      <Input
                        type="text"
                        value={state.completionsUrl}
                        onChange={(e) => updateCompletionsUrl(state.id, e.target.value)}
                        placeholder="https://{res}.openai.azure.com/openai/v1/deployments/{deployment}/chat/completions?api-version=2024-10-21"
                        className="h-8 text-xs"
                      />
                    </div>
                  ) : null}

                  {state.preset.Name === "Custom API" && state.preset.Models.length > 0 && (
                    <div className="flex items-center gap-2">
                      <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                        Model
                      </label>
                      <DropdownMenu>
                        <DropdownMenuTrigger>
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-8 flex-1 justify-between text-xs"
                          >
                            {state.selectedModel || "Select model"}
                            <ChevronDown className="h-3 w-3 opacity-50" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent className="w-[var(--anchor-width)] max-h-[200px] overflow-y-auto">
                          <DropdownMenuGroup>
                            <DropdownMenuGroupLabel>Available Models</DropdownMenuGroupLabel>
                            {state.preset.Models.map((m) => (
                              <DropdownMenuItem
                                key={m}
                                onClick={() => updateSelectedModel(state.id, m)}
                                className="cursor-pointer"
                              >
                                {m}
                              </DropdownMenuItem>
                            ))}
                          </DropdownMenuGroup>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  )}

                  {state.preset.ExtraFields?.map((field) => (
                    <div key={field.Key} className="flex items-center gap-2">
                      <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                        {field.Label}
                      </label>
                      <Input
                        type="text"
                        value={state.extraFields[field.Key] || ""}
                        onChange={(e) =>
                          updateExtraField(state.id, field.Key, e.target.value)
                        }
                        placeholder={field.Placeholder}
                        className="h-8 text-xs"
                      />
                    </div>
                  ))}

                  <div className="flex justify-end gap-2 pt-1">
                    {state.configured && (
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => removeProvider(state)}
                      >
                        <Trash2 className="h-3 w-3 mr-1" />
                        Remove
                      </Button>
                    )}
                    <Button
                      variant="default"
                      size="sm"
                      onClick={() => saveProvider(state)}
                    >
                      <Save className="h-3 w-3 mr-1" />
                      {state.configured ? "Update" : "Save"}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          ))}

          <div className="mt-3 rounded-lg border border-border border-dashed bg-background">
            {showAddCustom ? (
              <div className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium text-foreground">Custom Provider</span>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => setShowAddCustom(false)}
                  >
                    <X className="h-3 w-3" />
                  </Button>
                </div>
                <div className="flex items-center gap-2">
                  <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                    Name
                  </label>
                  <Input
                    type="text"
                    value={customName}
                    onChange={(e) => setCustomName(e.target.value)}
                    placeholder="My Provider"
                    className="h-8 text-xs"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                    Base URL
                  </label>
                  <Input
                    type="text"
                    value={customBaseUrl}
                    onChange={(e) => setCustomBaseUrl(e.target.value)}
                    placeholder="https://api.example.com/v1"
                    className="h-8 text-xs"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                    Completions URL
                  </label>
                  <Input
                    type="text"
                    value={customCompletionsUrl}
                    onChange={(e) => setCustomCompletionsUrl(e.target.value)}
                    placeholder="https://api.example.com/v1/chat/completions"
                    className="h-8 text-xs"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                    API Key
                  </label>
                  <Input
                    type="password"
                    value={customApiKey}
                    onChange={(e) => setCustomApiKey(e.target.value)}
                    placeholder="sk-..."
                    className="h-8 text-xs"
                  />
                </div>
                <div className="flex items-center gap-2">
                  <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                    Auth Type
                  </label>
                  <select
                    value={customAuthType}
                    onChange={(e) => setCustomAuthType(e.target.value)}
                    className="h-8 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 text-xs outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                  >
                    <option value="bearer">Bearer</option>
                    <option value="api-key">API Key</option>
                    <option value="none">None</option>
                  </select>
                </div>
                <div className="flex items-center gap-2">
                  <label className="w-20 shrink-0 text-xs font-medium text-muted-foreground">
                    Model
                  </label>
                  <Input
                    type="text"
                    value={customModel}
                    onChange={(e) => setCustomModel(e.target.value)}
                    placeholder="gpt-4o"
                    className="h-8 text-xs"
                  />
                </div>
                <div className="flex justify-end">
                  <Button
                    variant="default"
                    size="sm"
                    onClick={addCustomProvider}
                    disabled={!customName || !customBaseUrl}
                  >
                    <Plus className="h-3 w-3 mr-1" />
                    Add Provider
                  </Button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setShowAddCustom(true)}
                className="flex w-full items-center gap-2 px-4 py-3 text-left text-sm text-muted-foreground hover:text-foreground"
              >
                <Plus className="h-4 w-4" />
                Add Custom Provider
              </button>
            )}
          </div>
        </div>

        <div className="flex items-center justify-between border-t border-border px-5 py-3">
          <p className="text-xs text-muted-foreground">
            API keys are stored locally in ~/.config/demios/providers.json
          </p>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={onClose}>
              Cancel
            </Button>
            <Button variant="default" size="sm" onClick={saveAll} disabled={saving}>
              {saved ? (
                <>
                  <Check className="h-3 w-3 mr-1" />
                  Saved
                </>
              ) : saving ? (
                <>
                  <RefreshCw className="h-3 w-3 mr-1 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Save className="h-3 w-3 mr-1" />
                  Save All
                </>
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

function ProviderIcon({ name }: { name: string }) {
  const iconMap: Record<string, string> = {
    OpenAI: "O",
    Anthropic: "A",
    "Amazon Bedrock": "B",
    "Azure OpenAI": "Az",
    "Google Vertex AI": "G",
    Cohere: "C",
    Mistral: "M",
    Kimi: "K",
    Groq: "Gr",
    DeepSeek: "D",
    Ollama: "Oll",
    "LM Studio": "LM",
    OpenRouter: "OR",
  }
  const colorMap: Record<string, string> = {
    OpenAI: "bg-emerald-500/15 text-emerald-600",
    Anthropic: "bg-orange-500/15 text-orange-600",
    "Amazon Bedrock": "bg-amber-500/15 text-amber-600",
    "Azure OpenAI": "bg-blue-500/15 text-blue-600",
    "Google Vertex AI": "bg-red-500/15 text-red-600",
    Cohere: "bg-purple-500/15 text-purple-600",
    Mistral: "bg-indigo-500/15 text-indigo-600",
    Kimi: "bg-pink-500/15 text-pink-600",
    Groq: "bg-cyan-500/15 text-cyan-600",
    DeepSeek: "bg-violet-500/15 text-violet-600",
    Ollama: "bg-stone-500/15 text-stone-600",
    "LM Studio": "bg-teal-500/15 text-teal-600",
    OpenRouter: "bg-sky-500/15 text-sky-600",
  }
  const letter = iconMap[name] || name.charAt(0)
  return (
    <div
      className={cn(
        "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-bold",
        colorMap[name] || "bg-muted text-muted-foreground"
      )}
    >
      {letter}
    </div>
  )
}