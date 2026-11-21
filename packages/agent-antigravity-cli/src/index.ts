/**
 * Antigravity CLI agent runtime integration spawning `agy` subprocesses.
 */
export { AntigravityCliClient, probeAntigravity, type AntigravityRunHandle } from "./client.js";
export { normalizeAntigravityEvent, parseAntigravityLine } from "./normalize.js";
