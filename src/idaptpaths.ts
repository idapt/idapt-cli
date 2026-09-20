
import { homedir } from "node:os";
import { join } from "node:path";

const APP_DIR = "idapt";

function configBase(): string {
  const home = homedir();
  if (process.platform === "win32") {
    return process.env.APPDATA ?? join(home, "AppData", "Roaming");
  }
  if (process.platform === "darwin") {
    return join(home, "Library", "Application Support");
  }
  return process.env.XDG_CONFIG_HOME ?? join(home, ".config");
}

function cacheBase(): string {
  const home = homedir();
  if (process.platform === "win32") {
    return process.env.LOCALAPPDATA ?? join(home, "AppData", "Local");
  }
  if (process.platform === "darwin") {
    return join(home, "Library", "Caches");
  }
  return process.env.XDG_CACHE_HOME ?? join(home, ".cache");
}

export function configDir(): string {
  return join(configBase(), APP_DIR);
}

export function cacheDir(): string {
  return join(cacheBase(), APP_DIR);
}
