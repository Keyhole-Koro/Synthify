import { describe, expect, it } from "vitest";
import { nextSemanticLevel, semanticLevelForZoom, stageMetrics } from "./semantic-zoom.js";

describe("semantic zoom levels", () => {
  it("moves from structure to implementation as a Portal grows", () => {
    expect(nextSemanticLevel("structure", 90)).toBe("structure");
    expect(nextSemanticLevel("structure", 112)).toBe("flow");
    expect(nextSemanticLevel("flow", 230)).toBe("implementation");
  });

  it("holds the level inside the hysteresis band", () => {
    expect(nextSemanticLevel("implementation", 210)).toBe("implementation");
    expect(nextSemanticLevel("implementation", 200)).toBe("flow");
    expect(nextSemanticLevel("flow", 100)).toBe("flow");
    expect(nextSemanticLevel("flow", 96)).toBe("structure");
  });

  it("reads the level from the canvas zoom", () => {
    expect(semanticLevelForZoom("flow", 0.45)).toBe("structure");
    expect(semanticLevelForZoom("flow", 1)).toBe("flow");
    expect(semanticLevelForZoom("flow", 1.45)).toBe("implementation");
  });
});

describe("stage metrics", () => {
  const frame = { width: 960, height: 700 };

  it("fits a Room whose flow needs more room than the frame", () => {
    const metrics = stageMetrics([15, 32.5, 50, 67.5, 85].map((x) => ({ x, y: 50 })), frame);
    expect(metrics.scale).toBeLessThan(1);
    // Fitting a dense Room shrinks the projected Portal, which is what lowers
    // the semantic level rather than letting cards collide.
    expect(metrics.width * metrics.scale).toBeCloseTo(864, 5);
    expect(semanticLevelForZoom("flow", metrics.scale)).toBe("flow");
    const crowded = stageMetrics(Array.from({ length: 7 }, (_unused, index) => ({ x: 15 + index * 11.7, y: 50 })), frame);
    expect(semanticLevelForZoom("flow", crowded.scale)).toBe("structure");
  });

  it("enlarges a short flow so it gains detail instead of leaving the canvas empty", () => {
    const metrics = stageMetrics([{ x: 15, y: 50 }, { x: 50, y: 50 }, { x: 85, y: 50 }], frame);
    expect(metrics.scale).toBeGreaterThan(1);
    expect(semanticLevelForZoom("flow", metrics.scale)).toBe("implementation");
  });
});
