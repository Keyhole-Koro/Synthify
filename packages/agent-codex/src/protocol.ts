/** Unique identifier for a JSON-RPC request/response. */
export type JsonRpcId = number | string;

/**
 * Response payload returned by the JSON-RPC server.
 *
 * @property id Request identifier echoed by the server.
 * @property result Successful response payload, when the request completed normally.
 * @property error JSON-RPC error details when the request failed.
 */
export type JsonRpcResponse = {
  id: JsonRpcId;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
};

/**
 * One-way notification message with no expected reply.
 *
 * @property method Provider event or method name.
 * @property params Optional method-specific payload.
 */
export type JsonRpcNotification = {
  method: string;
  params?: unknown;
};

/**
 * Server-initiated request from the agent runtime requiring a client response (e.g. approval).
 *
 * @property id Identifier used to correlate the renderer's response.
 * @property method Requested operation name.
 * @property params Optional request-specific parameters.
 */
export type JsonRpcServerRequest = JsonRpcNotification & {
  id: JsonRpcId;
};

/** Union of all supported JSON-RPC message formats. */
export type JsonRpcMessage = JsonRpcResponse | JsonRpcNotification | JsonRpcServerRequest;

/**
 * Type guard verifying whether a value is a non-null, non-array object record.
 * @param value - Unknown input value.
 * @returns True if value is an object record.
 */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Type guard verifying whether a value conforms to the JSON-RPC message structure.
 * @param value - Unknown input value.
 * @returns True if value is a JsonRpcMessage.
 */
export function isJsonRpcMessage(value: unknown): value is JsonRpcMessage {
  if (!isRecord(value)) return false;
  return "id" in value || typeof value.method === "string";
}

/**
 * Parses a single line string into a validated JsonRpcMessage.
 * @param line - Raw JSON line from the process standard output.
 * @returns Parsed and validated JsonRpcMessage.
 * @throws Error if the parsed JSON does not conform to the JSON-RPC structure.
 */
export function parseJsonLine(line: string): JsonRpcMessage {
  const value: unknown = JSON.parse(line);
  if (!isJsonRpcMessage(value)) {
    throw new Error("Codex app-server emitted a non protocol JSON value");
  }
  return value;
}
