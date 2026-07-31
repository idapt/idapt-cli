

import {
  decodePtyExit,
  encodePtyData,
  encodePtyResize,
  PTY_FRAME_DATA,
  PTY_FRAME_EXIT,
  PtyFrameDecoder,
  TERMINAL_CLOSE_REASON,
  type TerminalCloseReason,
} from "@shared/computers/terminal-protocol";
import WebSocket from "ws";
import type { AgentToolsTransport } from "../transport";

export interface ShellOptions {

  readonly target: string;

  readonly tmux?: boolean;

  readonly runAs?: string;

  readonly workspace?: string;
}

export interface ShellIo {
  readonly stdin: NodeJS.ReadStream;
  readonly stdout: NodeJS.WriteStream;
  readonly err: (s: string) => void;
}

export function parseShellTarget(target: string): {
  computer: string;
  runAs?: string;
} {
  const at = target.lastIndexOf("@");
  if (at <= 0) return { computer: target };
  return { runAs: target.slice(0, at), computer: target.slice(at + 1) };
}

export function describeCloseReason(reason: string): string {
  switch (reason as TerminalCloseReason) {
    case TERMINAL_CLOSE_REASON.REMOTE_TERMINAL_POLICY_DISABLED:
      return (
        "This computer has interactive terminals disabled.\n" +
        "Run on that machine: idapt-computer service policy enable remote-terminal"
      );
    case TERMINAL_CLOSE_REASON.TUNNEL_POLICY_DISABLED:
      return (
        "This computer has tunnels disabled.\n" +
        "Run on that machine: idapt-computer service policy enable tunnels"
      );
    case TERMINAL_CLOSE_REASON.COMPUTER_OFFLINE:
      return "That computer is offline - start its daemon and try again.";
    case TERMINAL_CLOSE_REASON.TUNNEL_NOT_CONNECTED:
      return "That computer's daemon is running but not connected to the tunnel yet - retry in a moment.";
    case TERMINAL_CLOSE_REASON.TUNNEL_NOT_CONFIGURED:
      return "That computer has no tunnel configured - re-run `idapt-computer up` on it.";
    case TERMINAL_CLOSE_REASON.TUNNEL_PROXY_UNREACHABLE:
      return "Could not reach the Idapt tunnel proxy. Check your network and retry.";
    case TERMINAL_CLOSE_REASON.TUNNEL_PROXY_REJECTED:
      return "The tunnel proxy rejected the session. Your terminal token may have expired - retry.";

    case TERMINAL_CLOSE_REASON.TUNNEL_TOKEN_FAILED:
      return "That computer could not mint a tunnel token. Check `idapt-computer status` on it.";
    case TERMINAL_CLOSE_REASON.TUNNEL_SYNC_FAILED:
      return "That computer could not sync its tunnel state. Check `idapt-computer status` on it.";
    case TERMINAL_CLOSE_REASON.TUNNEL_SERVICE_FAILED:
      return "That computer's tunnel service failed to start the session. Check `idapt-computer status` on it.";
    case TERMINAL_CLOSE_REASON.TOO_MANY_TERMINALS:
      return "Too many terminals open on this account - close one and retry.";
    case TERMINAL_CLOSE_REASON.UNAUTHORIZED:
      return "Not signed in, or your session expired. Run `idapt login`.";
    case TERMINAL_CLOSE_REASON.FORBIDDEN:
      return "You do not have terminal access to that computer.";
    case TERMINAL_CLOSE_REASON.NOT_FOUND:
      return "No such computer.";
    case TERMINAL_CLOSE_REASON.CLOSED:
      return "";
    default:
      return reason ? `Session closed: ${reason}` : "Session closed.";
  }
}

interface PtyTokenResponse {
  token: string;
  proxyUrl: string;
  mode: string;
}

export async function runShell(
  transport: AgentToolsTransport,
  opts: ShellOptions,
  io: ShellIo,
): Promise<number> {
  const { computer, runAs: targetUser } = parseShellTarget(opts.target);
  const runAs = opts.runAs ?? targetUser;

  const query = new URLSearchParams({ mode: opts.tmux ? "tmux" : "shell" });
  if (runAs) query.set("runAs", runAs);
  if (opts.workspace) query.set("workspace", opts.workspace);

  const res = await transport.request({
    method: "GET",
    path: `/computers/${encodeURIComponent(computer)}/pty-token`,
    query: Object.fromEntries(query),
  });
  if (res.status >= 400 || !res.json) {
    const message =
      (res.json as { error?: { message?: string } } | undefined)?.error
        ?.message ??
      res.text ??
      `could not start a terminal (HTTP ${res.status})`;
    io.err(`✗ ${message}\n`);
    return res.status === 401 ? 2 : res.status === 403 ? 3 : 1;
  }

  const data = (res.json as { data?: PtyTokenResponse }).data;
  if (!data?.token || !data.proxyUrl) {
    io.err("✗ the server did not return a terminal token\n");
    return 1;
  }

  const cols = io.stdout.columns ?? 80;
  const rows = io.stdout.rows ?? 24;
  const wsUrl =
    `${data.proxyUrl.replace(/\/$/, "")}/__tunnel/pty?` +
    new URLSearchParams({
      mode: data.mode,
      cols: String(cols),
      rows: String(rows),
    }).toString();

  return openPtySession(wsUrl, data.token, io);
}

function openPtySession(
  wsUrl: string,
  token: string,
  io: ShellIo,
): Promise<number> {
  return new Promise((resolve) => {

    const socket = new WebSocket(wsUrl, {
      headers: { Authorization: `Bearer ${token}` },
    });
    socket.binaryType = "arraybuffer";

    const decoder = new PtyFrameDecoder();
    let exitCode = 0;
    let settled = false;
    let rawModeEngaged = false;

    const restore = () => {
      if (rawModeEngaged && io.stdin.isTTY) {
        try {
          io.stdin.setRawMode(false);
        } catch {

        }
      }
      io.stdin.pause();
      io.stdout.removeListener("resize", onResize);
      io.stdin.removeListener("data", onStdin);
    };

    const settle = (code: number) => {
      if (settled) return;
      settled = true;
      restore();
      resolve(code);
    };

    function onStdin(chunk: Buffer) {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(encodePtyData(new Uint8Array(chunk)));
      }
    }

    function onResize() {
      if (socket.readyState !== WebSocket.OPEN) return;
      socket.send(
        encodePtyResize(io.stdout.columns ?? 80, io.stdout.rows ?? 24),
      );
    }

    socket.on("open", () => {
      if (io.stdin.isTTY) {

        io.stdin.setRawMode(true);
        rawModeEngaged = true;
      }
      io.stdin.resume();
      io.stdin.on("data", onStdin);

      io.stdout.on("resize", onResize);
    });

    socket.on("message", (raw: ArrayBuffer | Buffer) => {
      const bytes =
        raw instanceof ArrayBuffer ? new Uint8Array(raw) : new Uint8Array(raw);
      let frames: ReturnType<PtyFrameDecoder["push"]>;
      try {
        frames = decoder.push(bytes);
      } catch (err) {
        io.err(`\n✗ ${(err as Error).message}\n`);
        socket.close();
        return;
      }
      for (const frame of frames) {
        if (frame.type === PTY_FRAME_DATA) {
          io.stdout.write(Buffer.from(frame.payload));
        } else if (frame.type === PTY_FRAME_EXIT) {
          exitCode = decodePtyExit(frame.payload) ?? 0;
        }
      }
    });

    socket.on("error", (err: Error) => {
      io.err(`\n✗ ${err.message}\n`);
      settle(1);
    });

    socket.on("close", (_code: number, reasonBuf: Buffer) => {
      const reason = reasonBuf?.toString() ?? "";
      const message = describeCloseReason(reason);
      if (message) io.err(`\n${message}\n`);

      settle(exitCode !== 0 ? exitCode : message ? 1 : 0);
    });
  });
}
