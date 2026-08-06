

import { IdaptError, NetworkError, RateLimitError } from "@idapt/sdk";
import { isUsageWallCode, type UsageWallCode } from "@shared/errors/wall-kind";
import { TransportTimeoutError } from "./transport";

function commandResource(command: string): string {
  return command.split(" ")[0] ?? command;
}

export type CredentialOrigin = "flag" | "env" | "file" | "none";

export interface ErrorContext {

  readonly credentialOrigin?: CredentialOrigin;

  readonly command?: string;

  readonly baseUrl?: string;

  readonly failure?: "user" | "packaging";

  readonly gatedFeature?: { readonly flag: string; readonly enabled: boolean };
}

export interface FormattedError {
  readonly message: string;
  readonly hints: readonly string[];
}

export function renderError(formatted: FormattedError): string {
  if (formatted.hints.length === 0) return formatted.message;
  return `${formatted.message}\n\n${formatted.hints.map((h) => `  ${h}`).join("\n")}`;
}

function wallHints(code: UsageWallCode): string[] {
  switch (code) {
    case "free_pool_exhausted":
      return [
        "Your free daily allowance is used up. It resets on a rolling window.",
        "Subscribe for a larger included allowance: https://idapt.app/pricing",
      ];
    case "subscription_required":
      return ["This needs a subscription: https://idapt.app/pricing"];
    case "plan_allowance_exhausted":
      return [
        "Your plan's included usage is spent.",
        "Turn on pay-as-you-go to keep going on your credit balance, or upgrade.",
      ];
    case "premium_allowance_exhausted":
      return [
        "Your weekly premium-model allowance is spent.",
        "Switch to a standard model, turn on pay-as-you-go, or upgrade.",
      ];
    case "computer_allowance_exhausted":
      return [
        "Your weekly cloud-computer allowance is spent.",
        "Running computers now bill your credit balance. Add credits or upgrade.",
      ];
    case "credits_exhausted":
      return ["Your credit balance is empty. Add credits to continue."];
    case "overflow_cap_reached":
      return ["You hit your pay-as-you-go monthly cap. Raise it to continue."];
    case "credit_spend_limit_exceeded":
      return ["This call exceeds your configured spend limit."];
    case "team_seat_cap":
      return ["Your team is at its seat cap. A billing admin can raise it."];
    case "team_budget":
      return ["Your team is at its budget. A billing admin can raise it."];
  }
}

function wallCodeOf(error: IdaptError): UsageWallCode | null {
  if (isUsageWallCode(error.subCode)) return error.subCode;
  const body = error.body as
    | { error?: { wall_code?: unknown; wallCode?: unknown } }
    | undefined;
  const raw = body?.error?.wall_code ?? body?.error?.wallCode;
  return isUsageWallCode(raw) ? raw : null;
}

function requestIdOf(error: IdaptError): string | null {
  const body = error.body as
    | { error?: { request_id?: unknown; requestId?: unknown } }
    | undefined;
  const raw = body?.error?.request_id ?? body?.error?.requestId;
  return typeof raw === "string" && raw.length > 0 ? raw : null;
}

function isPlanLimit(error: IdaptError): boolean {
  if (typeof error.subCode === "string" && /limit|quota/i.test(error.subCode)) {
    return true;
  }
  return /reached your limit of|plan (?:limit|includes)/i.test(error.message);
}

function isCredentialKindRefusal(error: IdaptError): boolean {
  return /not accessible via api key|session (?:auth|cookie) required/i.test(
    error.message,
  );
}

function invalidFieldsOf(error: IdaptError): string[] {
  const body = error.body as
    | {
        error?: {
          details?: unknown;
          issues?: unknown;
          fields?: Record<string, unknown>;
        };
      }
    | undefined;
  const node = body?.error;
  if (!node) return [];

  const out = new Set<string>();
  const issues = Array.isArray(node.details)
    ? node.details
    : Array.isArray(node.issues)
      ? node.issues
      : [];
  for (const issue of issues) {
    const path = (issue as { path?: unknown })?.path;
    if (Array.isArray(path) && path.length > 0) out.add(String(path[0]));
    else if (typeof path === "string" && path) out.add(path);
  }
  if (node.fields && typeof node.fields === "object") {
    for (const key of Object.keys(node.fields)) out.add(key);
  }
  return [...out];
}

function credentialSourceHint(origin: CredentialOrigin | undefined): string[] {
  switch (origin) {
    case "flag":
      return ["The key came from --api-key on this command line."];
    case "env":
      return [
        "The key came from IDAPT_API_KEY in your environment.",
        "That env var wins over a stored browser sign-in, so unset it if you meant to use `idapt login`.",
      ];
    case "file":
      return ["Run `idapt login` to sign in again."];
    default:
      return ["Run `idapt login`, or set IDAPT_API_KEY."];
  }
}

export function formatError(
  error: unknown,
  context: ErrorContext = {},
): FormattedError {
  if (error instanceof NetworkError || isFetchFailure(error)) {
    return {
      message: `Could not reach ${context.baseUrl ?? "the idapt API"}.`,
      hints: [
        "Check your connection. If you are on a VPN or corporate proxy, it may be blocking the request.",
        "If you meant to target a different server, pass --api-url or set IDAPT_API_URL.",
      ],
    };
  }

  if (error instanceof TransportTimeoutError) {
    return {
      message: prefixed(error.message),
      hints: [
        "The request may still be running on the server, so re-sending could duplicate it.",
        context.command
          ? `Check with \`idapt ${commandResource(context.command)} list\` before retrying.`
          : "Check the resource before retrying.",
      ],
    };
  }

  if (error instanceof IdaptError) {
    return formatIdaptError(error, context);
  }

  const message = error instanceof Error ? error.message : String(error);

  if (context.failure === "user") {
    return { message: prefixed(message), hints: [] };
  }

  if (context.failure === "packaging") {
    return { message: prefixed(message), hints: [] };
  }

  return {
    message: prefixed(message),
    hints: ["This looks like a bug. Re-run with --verbose for the full trace."],
  };
}

function prefixed(message: string): string {
  return message.startsWith("idapt: ") ? message : `idapt: ${message}`;
}

function formatIdaptError(
  error: IdaptError,
  context: ErrorContext,
): FormattedError {
  const hints: string[] = [];
  const requestId = requestIdOf(error);

  switch (error.status) {
    case 401:
      hints.push(...credentialSourceHint(context.credentialOrigin));
      break;
    case 403: {

      const wall = wallCodeOf(error);
      if (wall) {
        hints.push(...wallHints(wall));
        break;
      }
      if (isPlanLimit(error)) {
        hints.push(
          "This is a plan limit, not a permission problem.",
          "See what each plan includes: https://idapt.app/pricing",
        );
        break;
      }

      if (isCredentialKindRefusal(error)) {
        hints.push(
          "This command needs a browser sign-in; an API key cannot call it.",
          "Run `idapt login` (or `idapt login --device` over SSH), then retry.",
        );
        break;
      }
      hints.push(
        "You are authenticated but not allowed to do this. Retrying will not help.",
        "If this is in a shared workspace, ask an owner or admin for edit access.",
      );
      break;
    }
    case 404:

      if (context.gatedFeature && !context.gatedFeature.enabled) {
        hints.push(
          "This product is not enabled for your account yet, so the whole command is unavailable.",
          "Nothing is wrong with what you typed.",
        );
      } else {
        hints.push(
          "Check the id, name, or path. `idapt <resource> list` shows what you can reach.",
        );
      }
      break;
    case 402: {
      const wall = wallCodeOf(error);
      hints.push(
        ...(wall ? wallHints(wall) : ["Add credits or upgrade to continue."]),
      );
      break;
    }
    case 409:
      hints.push(
        "Something already exists, or the resource is in a state that blocks this.",
      );
      break;
    case 400:
    case 422: {

      const fields = invalidFieldsOf(error);
      if (fields.length > 0) {
        hints.push(
          `The server rejected: ${fields.map((f) => `--${f.replace(/_/g, "-")}`).join(", ")}.`,
        );
      }
      if (context.command) {
        hints.push(
          `Run \`idapt help ${context.command}\` for the full argument contract.`,
        );
      }
      break;
    }
    case 429: {
      const retryAfter =
        error instanceof RateLimitError ? error.retryAfter : undefined;
      hints.push(
        retryAfter
          ? `Rate limited. Retry in ${retryAfter} seconds.`
          : "Rate limited. Back off and retry.",
      );
      const wall = wallCodeOf(error);
      if (wall) hints.push(...wallHints(wall));
      break;
    }
    case 504:

      hints.push(
        "Nothing was lost and nothing needs re-sending. The work is still running.",
        context.command?.startsWith("chat ")
          ? "Read the chat back to collect the reply, or re-run with a longer --timeout."
          : "Check back with the resource's `get` verb, or re-run with a longer --timeout.",
      );
      break;
    default:
      if (error.status >= 500) {
        hints.push("The service failed. This is usually transient, so retry.");
      }
      break;
  }

  if (requestId) hints.push(`Request id: ${requestId}`);
  return { message: error.message, hints };
}

function isFetchFailure(error: unknown): boolean {
  return (
    error instanceof TypeError &&
    /fetch failed|network|ENOTFOUND|ECONNREFUSED|ETIMEDOUT/i.test(error.message)
  );
}
