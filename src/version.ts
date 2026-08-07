
export const VERSION: string = process.env.IDAPT_CLI_VERSION ?? "0.0.0-dev";

export const COMMIT: string = process.env.IDAPT_CLI_COMMIT ?? "unknown";
export const BUILT_AT: string = process.env.IDAPT_CLI_BUILT_AT ?? "unknown";

export const USER_AGENT = `idapt-cli/${VERSION}`;
