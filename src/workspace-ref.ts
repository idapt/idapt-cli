

export type WorkspaceOrigin = "flag" | "env" | "pin" | "default";

export interface EffectiveWorkspace {
  readonly ref: string;
  readonly origin: Exclude<WorkspaceOrigin, "default">;
}

export function effectiveWorkspace(args: {
  flag?: string | undefined;
  env?: Record<string, string | undefined> | undefined;
  pin?: string | undefined;
}): EffectiveWorkspace | null {
  if (args.flag) return { ref: args.flag, origin: "flag" };
  const fromEnv = args.env?.IDAPT_WORKSPACE;
  if (fromEnv) return { ref: fromEnv, origin: "env" };
  if (args.pin) return { ref: args.pin, origin: "pin" };
  return null;
}

export function originLabel(origin: WorkspaceOrigin): string {
  switch (origin) {
    case "flag":
      return "--workspace";
    case "env":
      return "IDAPT_WORKSPACE";
    case "pin":
      return "pinned with `idapt workspace use`";
    case "default":
      return "your account default";
  }
}
