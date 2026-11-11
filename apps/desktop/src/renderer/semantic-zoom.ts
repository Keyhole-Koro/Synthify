/**
 * Semantic Zoom changes which abstraction level a Portal presents, not how big
 * the card is. The level follows the projected width of a Portal, so zooming
 * out replaces detail with structure instead of shrinking unreadable text.
 */
export type SemanticLevel = "structure" | "flow" | "implementation";

/** Default layout width of a portal card in pixels. */
export const PORTAL_CARD_WIDTH = 190;

/** Projected pixel width thresholds for switching semantic zoom levels. */
export const SEMANTIC_THRESHOLDS = { flow: 112, implementation: 230 } as const;

/**
 * Interaction spec 9.3: a 12% hysteresis band keeps the level from flipping every frame
 * while the user hovers on a threshold boundary.
 */
const HYSTERESIS = 0.12;

/**
 * Computes the next semantic level based on projected width and current level with hysteresis.
 *
 * @param current The current semantic level.
 * @param projectedWidth The projected screen width of the card in pixels.
 * @returns The resolved semantic level.
 */
export function nextSemanticLevel(current: SemanticLevel, projectedWidth: number): SemanticLevel {
  const dropsBelow = (threshold: number) => projectedWidth < threshold * (1 - HYSTERESIS);
  if (projectedWidth >= SEMANTIC_THRESHOLDS.implementation) return "implementation";
  if (current === "implementation" && !dropsBelow(SEMANTIC_THRESHOLDS.implementation)) return "implementation";
  if (projectedWidth >= SEMANTIC_THRESHOLDS.flow) return "flow";
  if (current !== "structure" && !dropsBelow(SEMANTIC_THRESHOLDS.flow)) return "flow";
  return "structure";
}

/**
 * Resolves the semantic level for a given zoom factor relative to the base portal card width.
 *
 * @param current The current semantic level.
 * @param zoom Zoom multiplier applied to the stage.
 * @returns The resulting semantic level.
 */
export function semanticLevelForZoom(current: SemanticLevel, zoom: number): SemanticLevel {
  return nextSemanticLevel(current, PORTAL_CARD_WIDTH * zoom);
}

/** Column pitch spacing in pixels between adjacent portal columns. */
export const COLUMN_PITCH = PORTAL_CARD_WIDTH + 20;

/** Row pitch spacing in pixels between adjacent portal rows. */
export const ROW_PITCH = 196;

/** Dimensions and fit scale computed for the canvas stage. */
export type StageMetrics = {
  /** Total calculated width of the layout stage. */
  width: number;
  /** Total calculated height of the layout stage. */
  height: number;
  /** Scale factor to fit stage within frame bounds. */
  scale: number;
};

/**
 * layoutFlowNodes keeps the outermost column at 15% of the stage, so the stage
 * must be wide enough for half a card to fit inside that margin.
 */
const MINIMUM_STAGE_WIDTH = PORTAL_CARD_WIDTH / 0.3;

/**
 * Calculates stage dimension bounds and scale factor to fit a collection of nodes into a viewport frame.
 * A Room is laid out at its natural pitch and then fitted into the frame, so a dense Room reads
 * at a lower semantic level instead of overlapping itself.
 *
 * @param nodes Coordinates of all active nodes in normalized space.
 * @param frame Viewport dimensions of the canvas frame.
 * @returns Calculated stage width, height, and fit scale.
 */
export function stageMetrics(nodes: Array<{ x: number; y: number }>, frame: { width: number; height: number }): StageMetrics {
  const inner = { width: Math.max(320, frame.width - 96), height: Math.max(240, frame.height - 96) };
  if (nodes.length === 0) return { width: inner.width, height: inner.height, scale: 1 };
  const xs = nodes.map((node) => node.x);
  const ys = nodes.map((node) => node.y);
  const columns = new Set(xs.map(Math.round)).size;
  const rows = new Set(ys.map(Math.round)).size;
  const spanX = Math.max(1, Math.max(...xs) - Math.min(...xs));
  const spanY = Math.max(1, Math.max(...ys) - Math.min(...ys));
  const width = Math.max(MINIMUM_STAGE_WIDTH, columns > 1 ? ((columns - 1) * COLUMN_PITCH * 100) / spanX : COLUMN_PITCH * 2);
  const height = rows > 1 ? ((rows - 1) * ROW_PITCH * 100) / spanY : ROW_PITCH * 2;
  // A Room that needs less space than the frame is shown larger, so a short
  // flow gains detail instead of leaving the canvas empty.
  const scale = Math.min(1.35, Math.max(0.3, Math.min(inner.width / width, inner.height / height)));
  return { width, height, scale };
}
