import {
  CircleAlert,
  CircleCheck,
  CircleSlash,
  LoaderCircle,
  type LucideIcon,
} from "lucide-react"

import { isTerminal, type JobStatus } from "@/lib/jobs"
import { jobStatusLabels } from "@/lib/messages"
import { cn } from "@/lib/utils"

const statusIcon: Record<JobStatus, { icon: LucideIcon; className: string }> = {
  queued: { icon: LoaderCircle, className: "text-muted-foreground" },
  processing: { icon: LoaderCircle, className: "text-muted-foreground" },
  submitted: { icon: LoaderCircle, className: "text-info" },
  printing: { icon: LoaderCircle, className: "text-info" },
  completed: { icon: CircleCheck, className: "text-success" },
  failed: { icon: CircleAlert, className: "text-destructive" },
  cancelled: { icon: CircleSlash, className: "text-muted-foreground" },
}

/** A job status as icon + text; never color alone. Active statuses spin. */
export function JobStatusLabel({
  status,
  size = "sm",
  className,
}: {
  status: JobStatus
  size?: "sm" | "lg"
  className?: string
}) {
  const { icon: Icon, className: tone } = statusIcon[status]
  return (
    <span
      className={cn(
        "flex items-center",
        size === "lg" ? "gap-2 text-lg font-medium" : "gap-1.5 text-sm",
        tone,
        className
      )}
    >
      <Icon
        aria-hidden
        className={cn(
          size === "lg" ? "size-5" : "size-4",
          "shrink-0",
          !isTerminal(status) && "motion-safe:animate-spin"
        )}
      />
      {jobStatusLabels[status]}
    </span>
  )
}
