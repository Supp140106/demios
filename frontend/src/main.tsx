import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import App from "./App.tsx"
import { ThemeProvider } from "@/components/theme-provider.tsx"
import { ErrorBoundary } from "@/components/error-boundary.tsx"
import { Toaster } from "@/components/ui/sonner"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary>
      <ThemeProvider>
        <App />
        <Toaster
          position="top-center"
          richColors
          closeButton
          toastOptions={{ duration: 6000 }}
        />
      </ThemeProvider>
    </ErrorBoundary>
  </StrictMode>
)
