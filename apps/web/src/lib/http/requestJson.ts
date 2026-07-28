import { ZodError, type ZodType } from "zod";

import { environment } from "../../config/env";

export class ApiError extends Error {
  public constructor(
    message: string,
    public readonly status: number,
    public readonly requestId: string | null,
    public readonly details: unknown = null,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export type RequestJsonOptions<TResponse> = Omit<RequestInit, "body" | "signal"> & {
  readonly schema: ZodType<TResponse>;
  readonly body?: unknown;
  readonly signal?: AbortSignal;
  readonly timeoutMs?: number;
};

async function readResponseBody(response: Response): Promise<unknown> {
  const contentType = response.headers.get("content-type") ?? "";

  if (contentType.includes("application/json")) {
    return response.json();
  }

  const text = await response.text();

  return text === "" ? null : text;
}

function extractErrorMessage(payload: unknown, fallback: string): string {
  if (
    typeof payload === "object" &&
    payload !== null &&
    "message" in payload &&
    typeof payload.message === "string"
  ) {
    return payload.message;
  }

  if (typeof payload === "string" && payload !== "") {
    return payload;
  }

  return fallback;
}

export async function requestJson<TResponse>(
  path: string,
  options: RequestJsonOptions<TResponse>,
): Promise<TResponse> {
  const { schema, body, timeoutMs = 15_000, signal, headers, ...requestOptions } = options;

  const controller = new AbortController();

  const forwardAbort = (): void => {
    controller.abort(signal?.reason);
  };

  if (signal?.aborted) {
    forwardAbort();
  } else {
    signal?.addEventListener("abort", forwardAbort, {
      once: true,
    });
  }

  const timeout = globalThis.setTimeout(() => {
    controller.abort(new DOMException("Request timed out", "TimeoutError"));
  }, timeoutMs);

  const requestHeaders = new Headers(headers);

  requestHeaders.set("Accept", "application/json");

  if (body !== undefined && !requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", "application/json");
  }

  const requestInit: RequestInit = {
    ...requestOptions,
    credentials: "include",
    headers: requestHeaders,
    signal: controller.signal,
  };

  if (body !== undefined) {
    requestInit.body = JSON.stringify(body);
  }

  try {
    const response = await fetch(`${environment.apiBaseUrl}${path}`, requestInit);

    const payload = await readResponseBody(response);
    const requestId = response.headers.get("x-request-id");

    if (!response.ok) {
      throw new ApiError(
        extractErrorMessage(payload, `Request failed with status ${response.status}`),
        response.status,
        requestId,
        payload,
      );
    }

    try {
      return schema.parse(payload);
    } catch (error) {
      if (error instanceof ZodError) {
        throw new ApiError("The API returned an invalid response", 502, requestId, error.issues);
      }

      throw error;
    }
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }

    if (controller.signal.aborted) {
      if (signal?.aborted) {
        throw new ApiError("The request was canceled", 499, null, signal.reason);
      }

      throw new ApiError("The request timed out", 408, null);
    }

    throw new ApiError(
      error instanceof Error ? error.message : "A network error occurred",
      0,
      null,
      error,
    );
  } finally {
    globalThis.clearTimeout(timeout);

    signal?.removeEventListener("abort", forwardAbort);
  }
}
