# 0006: DOM and SVG FlowFold Renderer

**Status:** Accepted  
**Date:** 2026-08-28

## Context

FlowFold は text editing、Thread composer、Portal、Peek、selection、keyboard navigationを含む。RoomとSemantic Zoomにより、
一度に表示するNode数を制御できる。

## Decision

MVPはNode/Portal/ThreadをDOM、Edgeとselection overlayをSVGで描画する。layoutはpure TypeScript + Web Worker、
Peekはchild Roomの再帰mountではなくsnapshot representationとする。Portal外形をchild layout sizeから独立させる。

## Alternatives

- Canvas/WebGL: 大規模描画に強いがtext/accessibility/testingの実装負担が増える。
- Existing paper-in-paperそのまま: interaction仮説には有用だが余白と再帰mountの制約を継承する。

## Consequences

- DesktopとWeb companionでrendererを共有できる。
- Room内500 visible nodesまたはframe budget超過時にWebGL layerを再評価する。
- paper-in-paperはreference prototypeとして保持する。
