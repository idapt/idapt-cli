

import {
  type HttpContext,
  IDAPT_API_VERSION,
  IDAPT_API_VERSION_HEADER,
  IdaptError,
  requestRaw,
} from "@idapt/sdk";

export interface AgentToolsRequest {
  readonly method: "GET" | "POST" | "PATCH" | "PUT" | "DELETE";

  readonly path: string;
  readonly query?: Record<string, string | number | boolean | undefined>;

  readonly body?: Record<string, unknown>;

  readonly multipart?: Record<string, string | Blob>;

  readonly expectBinary?: boolean;
  readonly signal?: AbortSignal;
}

export interface AgentToolsResponse {
  readonly status: number;

  readonly json?: unknown;

  readonly text?: string;

  readonly blob?: Blob;
}

export interface AgentToolsTransport {
  request(req: AgentToolsRequest): Promise<AgentToolsResponse>;

  readonly ctx?: HttpContext;
}

export interface FetchTransportOptions {

  readonly baseUrl: string;

  readonly token: string;

  readonly fetch?: typeof fetch;

  readonly userAgent?: string;

  readonly timeoutMs?: number;

  readonly onTrace?: (line: string) => void;
}

async function decodeOk(
  res: Response,
  expectBinary: boolean | undefined,
): Promise<AgentToolsResponse> {
  const ct = res.headers.get("content-type") ?? "";
  if (expectBinary) return { status: res.status, blob: await res.blob() };
  if (ct.includes("application/json")) {
    return { status: res.status, json: await res.json() };
  }
  return { status: res.status, text: await res.text() };
}

function decodeError(err: IdaptError): AgentToolsResponse {
  if (typeof err.body === "string") {
    return { status: err.status, text: err.body };
  }
  if (err.body !== undefined && err.body !== null) {
    return { status: err.status, json: err.body };
  }

  return { status: err.status, text: err.message };
}

export function createFetchTransport(
  opts: FetchTransportOptions,
): AgentToolsTransport {

  const ctx: HttpContext = {
    apiUrl: opts.baseUrl,
    key: opts.token,
    headers: {
      [IDAPT_API_VERSION_HEADER]: IDAPT_API_VERSION,
      ...(opts.userAgent ? { "User-Agent": opts.userAgent } : {}),
    },
    ...(opts.fetch ? { fetch: wrapFetch(opts.fetch, opts) } : {}),
    ...(!opts.fetch && (opts.timeoutMs || opts.onTrace)
      ? { fetch: wrapFetch(globalThis.fetch, opts) }
      : {}),
  };
  return {
    ctx,
    async request(req: AgentToolsRequest): Promise<AgentToolsResponse> {

      let bodyRaw: BodyInit | undefined;
      if (req.multipart) {
        const form = new FormData();
        for (const [k, v] of Object.entries(req.multipart)) form.set(k, v);
        bodyRaw = form;
      }
      try {
        const res = await requestRaw(ctx, {
          method: req.method,
          path: req.path,
          ...(req.query ? { query: req.query } : {}),

          ...(bodyRaw === undefined && req.body !== undefined
            ? { body: req.body }
            : {}),
          ...(bodyRaw !== undefined ? { bodyRaw } : {}),

          ...(req.signal ? { signal: req.signal } : {}),

          expectJson: false,
        });
        return decodeOk(res, req.expectBinary);
      } catch (err) {

        if (err instanceof IdaptError && err.status >= 400) {
          return decodeError(err);
        }
        throw err;
      }
    },
  };
}

export class TransportTimeoutError extends Error {
  readonly timeoutMs: number;
  constructor(timeoutMs: number) {
    super(
      `Timed out after ${timeoutMs / 1000}s waiting for the server. ` +
        "Raise it with --timeout <seconds>.",
    );
    this.name = "TransportTimeoutError";
    this.timeoutMs = timeoutMs;
  }
}

function isAbort(error: unknown): boolean {
  return (
    (error instanceof Error &&
      (error.name === "AbortError" || error.name === "TimeoutError")) ||
    (error instanceof DOMException && error.name === "AbortError")
  );
}

function wrapFetch(
  inner: typeof fetch,
  opts: FetchTransportOptions,
): typeof fetch {
  return (async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const started = Date.now();
    const method = (init.method ?? "GET").toUpperCase();
    const url =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url;

    let signal = init.signal ?? undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    if (opts.timeoutMs && opts.timeoutMs > 0) {
      const controller = new AbortController();
      timer = setTimeout(() => controller.abort(), opts.timeoutMs);

      signal?.addEventListener("abort", () => controller.abort(), {
        once: true,
      });
      signal = controller.signal;
    }

    try {
      const res = await inner(input, {
        ...init,
        ...(signal ? { signal } : {}),
      });
      opts.onTrace?.(
        `${method} ${url} -> ${res.status} (${Date.now() - started}ms)`,
      );
      if (opts.onTrace && !res.ok) {

        const body = await res
          .clone()
          .text()
          .catch(() => "");
        if (body) opts.onTrace(`  body: ${body.slice(0, 2000)}`);
      }
      return res;
    } catch (error) {

      if (isAbort(error) && opts.timeoutMs && opts.timeoutMs > 0) {
        opts.onTrace?.(
          `${method} ${url} -> timed out after ${opts.timeoutMs}ms`,
        );
        throw new TransportTimeoutError(opts.timeoutMs);
      }
      opts.onTrace?.(
        `${method} ${url} -> failed (${Date.now() - started}ms): ${
          error instanceof Error ? error.message : String(error)
        }`,
      );
      throw error;
    } finally {
      if (timer) clearTimeout(timer);
    }
  }) as typeof fetch;
}
