

export const MARKED_BEGIN = "# >>> idapt >>>";
export const MARKED_END = "# <<< idapt <<<";

export interface MarkedBlockResult {
  text: string;
  changed: boolean;
}

export function renderMarkedBlock(body: string): string {
  const inner = body.replace(/\n+$/, "");
  return `${MARKED_BEGIN}\n${inner}\n${MARKED_END}\n`;
}

export function upsertMarkedBlock(
  existing: string,
  body: string,
): MarkedBlockResult {
  const block = renderMarkedBlock(body);
  const beginIdx = existing.indexOf(MARKED_BEGIN);
  const endIdx = existing.indexOf(MARKED_END);

  if (beginIdx !== -1 && endIdx !== -1 && endIdx > beginIdx) {
    const before = existing.slice(0, beginIdx);
    const after = existing.slice(endIdx + MARKED_END.length);

    const afterTrimmed = after.replace(/^\n/, "");
    const next = `${before}${block}${afterTrimmed}`;
    return { text: next, changed: next !== existing };
  }

  let out = existing;
  if (out.length > 0 && !out.endsWith("\n")) out += "\n";
  if (out.trim().length > 0) out += "\n";
  out += block;
  return { text: out, changed: true };
}

export function removeMarkedBlock(existing: string): MarkedBlockResult {
  const beginIdx = existing.indexOf(MARKED_BEGIN);
  const endIdx = existing.indexOf(MARKED_END);
  if (beginIdx === -1 || endIdx === -1 || endIdx < beginIdx) {
    return { text: existing, changed: false };
  }
  const before = existing.slice(0, beginIdx);
  const after = existing.slice(endIdx + MARKED_END.length).replace(/^\n/, "");
  return { text: `${before}${after}`, changed: true };
}
