import { Select as SelectPrimitive } from "@base-ui/react/select"
import { cn } from "@/lib/utils"
import { Check, ChevronDown } from "lucide-react"

function SelectRoot<Value, Multiple extends boolean | undefined = false>(
  props: SelectPrimitive.Root.Props<Value, Multiple>
) {
  return <SelectPrimitive.Root {...props} />
}

function SelectTrigger({
  className,
  children,
  ...props
}: SelectPrimitive.Trigger.Props) {
  return (
    <SelectPrimitive.Trigger
      className={cn(
        "flex h-7 w-full items-center gap-1.5 truncate rounded-md border border-border bg-transparent px-2 py-1 text-[11px] text-muted-foreground",
        "hover:bg-muted focus:ring-1 focus:ring-ring focus:outline-none",
        "cursor-pointer transition-colors",
        className
      )}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon className="ml-auto shrink-0 opacity-50">
        <ChevronDown className="h-3 w-3" />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  )
}

function SelectValue({
  className,
  children,
  ...props
}: SelectPrimitive.Value.Props) {
  return (
    <SelectPrimitive.Value className={cn("truncate", className)} {...props}>
      {children}
    </SelectPrimitive.Value>
  )
}

function SelectPopup({ className, ...props }: SelectPrimitive.Popup.Props) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Positioner
        className="z-50 w-[--anchor-width]"
        sideOffset={4}
      >
        <SelectPrimitive.Popup
          className={cn(
            "origin-[--anchor-transform-origin] overflow-hidden rounded-lg border border-border bg-popover p-1 shadow-lg shadow-black/5",
            "transition-[transform,scale,opacity] data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0",
            className
          )}
          {...props}
        />
      </SelectPrimitive.Positioner>
    </SelectPrimitive.Portal>
  )
}

function SelectList({ className, ...props }: SelectPrimitive.List.Props) {
  return (
    <SelectPrimitive.List
      className={cn("flex flex-col gap-0.5", className)}
      {...props}
    />
  )
}

function SelectItem({
  className,
  children,
  ...props
}: SelectPrimitive.Item.Props) {
  return (
    <SelectPrimitive.Item
      className={cn(
        "relative flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-xs outline-none select-none",
        "data-[highlighted]:bg-muted data-[highlighted]:text-foreground",
        "data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        className
      )}
      {...props}
    >
      <SelectPrimitive.ItemText className="flex-1 truncate">
        {children}
      </SelectPrimitive.ItemText>
      <SelectPrimitive.ItemIndicator className="ml-auto shrink-0">
        <Check className="h-3 w-3 text-primary" />
      </SelectPrimitive.ItemIndicator>
    </SelectPrimitive.Item>
  )
}

export const Select = {
  Root: SelectRoot,
  Trigger: SelectTrigger,
  Value: SelectValue,
  Popup: SelectPopup,
  List: SelectList,
  Item: SelectItem,
}
