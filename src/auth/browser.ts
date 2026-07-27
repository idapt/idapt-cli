
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

export function openBrowser(url: string): boolean {
  try {
    const platform = process.platform;
    const cmd =
      platform === "darwin"
        ? "open"
        : platform === "win32"
          ? "cmd"
          : "xdg-open";
    const args = platform === "win32" ? ["/c", "start", "", url] : [url];
    const child = spawn(cmd, args, { stdio: "ignore", detached: true });
    child.on("error", () => {

    });
    child.unref();
    return true;
  } catch {
    return false;
  }
}
