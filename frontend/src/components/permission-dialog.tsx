import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { ShieldAlert, ShieldCheck, ShieldX } from "lucide-react"
import type { PermissionRequest } from "@/hooks/use-permission"

type PermissionDialogProps = {
  request: PermissionRequest | null
  onAllow: () => void
  onDeny: () => void
}

export function PermissionDialog({
  request,
  onAllow,
  onDeny,
}: PermissionDialogProps) {
  return (
    <AlertDialog
      open={request !== null}
      onOpenChange={(open) => !open && onDeny()}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <ShieldAlert className="h-5 w-5 text-amber-500" />
            Permission Request
          </AlertDialogTitle>
          <AlertDialogDescription>
            <div className="space-y-3 pt-2">
              <p>The agent wants to execute the following tool:</p>
              <div className="rounded-md border bg-muted/50 p-3 font-mono text-sm">
                <div className="flex items-center gap-2">
                  {request?.name === "Write" || request?.name === "Edit" ? (
                    <ShieldX className="h-4 w-4 text-red-400" />
                  ) : (
                    <ShieldCheck className="h-4 w-4 text-green-400" />
                  )}
                  <span className="font-semibold">{request?.name}</span>
                </div>
                {request?.args && (
                  <pre className="mt-2 overflow-x-auto text-xs text-muted-foreground">
                    {JSON.stringify(request.args, null, 2)}
                  </pre>
                )}
              </div>
              <p className="text-sm text-muted-foreground">
                {request?.name === "Write"
                  ? "This will create or overwrite a file."
                  : request?.name === "Edit"
                    ? "This will modify an existing file."
                    : "This will execute a command on your system."}
              </p>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" onClick={onDeny}>
            Deny
          </Button>
          <Button variant="default" onClick={onAllow}>
            Allow
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
