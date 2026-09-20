

import { openAsBlob } from "node:fs";
import { stat } from "node:fs/promises";
import { basename } from "node:path";

class StreamingFile extends File {
  readonly #inner: Blob;
  readonly #onBytes: ((sent: number) => void) | undefined;

  constructor(inner: Blob, name: string, onBytes?: (sent: number) => void) {

    super([inner], name, { type: inner.type });
    this.#inner = inner;
    this.#onBytes = onBytes;
  }

  slice(start?: number, end?: number, contentType?: string): Blob {
    return this.#inner.slice(start, end, contentType);
  }

  arrayBuffer(): Promise<ArrayBuffer> {
    return this.#inner.arrayBuffer();
  }

  bytes(): ReturnType<Blob["bytes"]> {
    return this.#inner.bytes();
  }

  text(): Promise<string> {
    return this.#inner.text();
  }

  stream(): ReturnType<Blob["stream"]> {
    const source = this.#inner.stream();
    const onBytes = this.#onBytes;
    if (!onBytes) return source;
    let sent = 0;
    return source.pipeThrough(
      new TransformStream({
        transform(chunk, controller) {
          sent += chunk.byteLength;
          onBytes(sent);
          controller.enqueue(chunk);
        },
      }),
    );
  }
}

export interface LocalFilePart {

  readonly file: File;

  readonly size: number;
  readonly name: string;
}

export async function openLocalFilePart(
  path: string,
  onBytes?: (sent: number) => void,
): Promise<LocalFilePart> {
  let size: number;
  try {
    const info = await stat(path);
    if (info.isDirectory()) {
      throw new DirectoryFileError(
        `--file ${path} is a directory. Name a single file, or use \`computer upload\` to send a tree.`,
      );
    }
    size = info.size;
  } catch (error) {
    if (error instanceof DirectoryFileError) throw error;
    throw new Error(`--file ${path} could not be read: no such file.`);
  }
  const blob = await openAsBlob(path);
  return {
    file: new StreamingFile(blob, basename(path), onBytes),
    size,
    name: basename(path),
  };
}

class DirectoryFileError extends Error {}
