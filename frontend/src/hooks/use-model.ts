import { useState, useEffect, useCallback } from "react"
import { GetModels, SetModel, GetCurrentModel } from "../../wailsjs/go/main/App"
import type { llm } from "../../wailsjs/go/models"

export type ModelInfo = llm.ModelConfig

const STORAGE_KEY = "demios-selected-model"

export function useModel() {
  const [models, setModels] = useState<ModelInfo[]>([])
  const [currentModel, setCurrentModel] = useState<string>("")
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    async function init() {
      try {
        const [modelList, currentId] = await Promise.all([
          GetModels(),
          GetCurrentModel(),
        ])
        setModels(modelList)
        setCurrentModel(currentId)
        setLoaded(true)
      } catch {
        setLoaded(true)
      }
    }
    init()
  }, [])

  const selectModel = useCallback(async (id: string) => {
    try {
      await SetModel(id)
      setCurrentModel(id)
      localStorage.setItem(STORAGE_KEY, id)
    } catch (err) {
      console.error("Failed to switch model:", err)
    }
  }, [])

  return { models, currentModel, selectModel, loaded }
}
