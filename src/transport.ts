

import { type HttpContext, IdaptError, requestRaw } from "@idapt/sdk";

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
    ...(opts.userAgent ? { headers: { "User-Agent": opts.userAgent } } : {}),
    ...(opts.fetch ? { fetch: opts.fetch } : {}),
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
