

export interface AgentToolsRequest {
  readonly method: "GET" | "POST" | "PATCH" | "DELETE";

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
}

export interface FetchTransportOptions {

  readonly baseUrl: string;

  readonly token: string;

  readonly fetch?: typeof fetch;

  readonly userAgent?: string;
}

function buildUrl(
  baseUrl: string,
  path: string,
  query?: AgentToolsRequest["query"],
): string {
  const base = baseUrl.replace(/\/+$/, "");
  const url = `${base}${path.startsWith("/") ? path : `/${path}`}`;
  if (!query) return url;
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined) sp.set(k, String(v));
  }
  const qs = sp.toString();
  return qs ? `${url}?${qs}` : url;
}

export function createFetchTransport(
  opts: FetchTransportOptions,
): AgentToolsTransport {
  const doFetch = opts.fetch ?? globalThis.fetch;
  return {
    async request(req: AgentToolsRequest): Promise<AgentToolsResponse> {
      const headers: Record<string, string> = {
        Authorization: `Bearer ${opts.token}`,
      };
      if (opts.userAgent) headers["User-Agent"] = opts.userAgent;
      let body: BodyInit | undefined;
      if (req.multipart) {
        const form = new FormData();
        for (const [k, v] of Object.entries(req.multipart)) form.set(k, v);
        body = form;
      } else if (req.body !== undefined) {
        headers["Content-Type"] = "application/json";
        body = JSON.stringify(req.body);
      }
      const res = await doFetch(buildUrl(opts.baseUrl, req.path, req.query), {
        method: req.method,
        headers,
        body,
        signal: req.signal,
      });
      const ct = res.headers.get("content-type") ?? "";
      if (req.expectBinary && res.ok) {
        return { status: res.status, blob: await res.blob() };
      }
      if (ct.includes("application/json")) {
        return { status: res.status, json: await res.json() };
      }
      return { status: res.status, text: await res.text() };
    },
  };
}
