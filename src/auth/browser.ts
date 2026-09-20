
import { spawn } from "node:child_process";

export function canOpenBrowser(
  env: Record<string, string | undefined>,
  isTty: boolean,
): boolean {
  if (!isTty) return false;

  if (env.SSH_CONNECTION || env.SSH_TTY) return false;
  if (process.platform === "linux") {
    return Boolean(env.DISPLAY || env.WAYLAND_DISPLAY);
  }

  return true;
}

export function windowsOpenCommand(url: string): {
  file: string;
  args: string[];
} {
  const escaped = url.replace(/[&|^<>()%!"]/g, (c) => `^${c}`);
  return {
    file: process.env.ComSpec || "cmd.exe",
    args: ["/d", "/s", "/c", "start", '""', escaped],
  };
}

export function openCommandFor(
  url: string,
  platform: NodeJS.Platform = process.platform,
): { file: string; args: string[]; verbatim: boolean } {
  if (platform === "win32") {
    return { ...windowsOpenCommand(url), verbatim: true };
  }
  return {
    file: platform === "darwin" ? "open" : "xdg-open",
    args: [url],
    verbatim: false,
  };
}

export function openBrowser(url: string): boolean {
  try {
    const { file, args, verbatim } = openCommandFor(url);
    const child = spawn(file, args, {
      stdio: "ignore",
      detached: true,

      ...(verbatim ? { windowsVerbatimArguments: true } : {}),
    });
    child.on("error", () => {

    });
    child.unref();
    return true;
  } catch {
    return false;
  }
}
