import { buildPaperMap } from '@keyhole-koro/paper-in-paper';
import type { Paper, PaperMap } from '@keyhole-koro/paper-in-paper';

const papers: Paper[] = [
  {
    id: 'root',
    title: 'Paper in Paper',
    description: 'Recursive paper demo root.',
    hue: 36,
    content: `
      <section class="hero-block">
        <p class="eyebrow">Interactive document playground</p>
        <h1>Paper in Paper</h1>
        <p class="lede">
          Paper in Paper is a reading UI where concepts expand in place.
          Start from <a data-paper-id="concept">concept</a>, inspect the
          <a data-paper-id="architecture">architecture</a>, or jump into the
          <a data-paper-id="interaction">interaction model</a>.
        </p>
        <div class="callout-grid">
          <div class="stat-card">
            <strong>Nested</strong>
            <span>Topics can open without losing the parent context.</span>
          </div>
          <div class="stat-card">
            <strong>Isolated</strong>
            <span>HTML lives in an iframe, so styles stay local.</span>
          </div>
        </div>
      </section>
      <section>
        <h2>What this demo exercises</h2>
        <ul class="check-list">
          <li><strong>Recursive expansion</strong> from links embedded in prose</li>
          <li><strong>Iframe click handling</strong> and resize reporting</li>
          <li><strong>Drag-and-drop</strong> between sibling rooms</li>
        </ul>
        <p>
          The layout is intentionally compact, but the content mixes
          <mark>headings</mark>, <code>inline code</code>, lists, and media so the
          renderer can be stressed a bit more than plain paragraphs.
        </p>
      </section>
    `,
    parentId: null,
    childIds: ['concept', 'architecture', 'interaction', 'notes'],
  },
  {
    id: 'concept',
    title: 'Concept',
    description: 'Why recursive reading is useful.',
    hue: 150,
    content: `
      <section>
        <h2>Why split reading into papers?</h2>
        <p>
          Instead of showing one long article, each paper can open a narrower topic.
          A reader can move from the main thread into
          <a data-paper-id="focus">focused reading</a> or compare it with
          <a data-paper-id="sidebar">sidebar staging</a>.
        </p>
        <blockquote>
          <p>
            Good navigation keeps the current thought visible while inviting a
            deeper branch only when the reader asks for it.
          </p>
        </blockquote>
        <ol>
          <li>Keep the parent idea visible.</li>
          <li>Open a more specific child when needed.</li>
          <li>Close the branch without losing place.</li>
        </ol>
      </section>
    `,
    parentId: 'root',
    childIds: ['focus', 'sidebar'],
  },
  {
    id: 'architecture',
    title: 'Architecture',
    description: 'React + TypeScript implementation slices.',
    hue: 215,
    content: `
      <section>
        <h2>Implementation slices</h2>
        <p>
          The implementation separates
          <a data-paper-id="core-state">core state</a>,
          <a data-paper-id="iframe-bridge">iframe bridge</a>, and
          <a data-paper-id="layout-engine">layout logic</a>.
        </p>
        <table>
          <thead>
            <tr>
              <th>Layer</th>
              <th>Responsibility</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Core</td>
              <td>Tree state, focus, open children, ordering.</td>
            </tr>
            <tr>
              <td>React</td>
              <td>Rooms, headers, recursive node rendering.</td>
            </tr>
            <tr>
              <td>Iframe</td>
              <td>Content isolation and link event bridging.</td>
            </tr>
          </tbody>
        </table>
      </section>
    `,
    parentId: 'root',
    childIds: ['core-state', 'iframe-bridge', 'layout-engine'],
  },
  {
    id: 'interaction',
    title: 'Interaction',
    description: 'Open, close, breadcrumbs, and drag.',
    hue: 338,
    content: `
      <section>
        <h2>Interaction model</h2>
        <p>
          Users open linked papers inline, close branches from breadcrumbs,
          and rearrange nodes with drag-and-drop. See
          <a data-paper-id="breadcrumbs">breadcrumbs</a> and
          <a data-paper-id="drag-drop">drag and drop</a>.
        </p>
        <div class="tip-box">
          <strong>Try this:</strong>
          click a linked chip, then drag another chip into a different room.
        </div>
      </section>
    `,
    parentId: 'root',
    childIds: ['breadcrumbs', 'drag-drop'],
  },
  {
    id: 'notes',
    title: 'Notes',
    description: 'Free-standing paper without inline links.',
    hue: 270,
    content: `
      <article>
        <h2>Mixed media paper</h2>
        <p>
          Papers do not need to expose every child through inline links.
          A room can still contain child cards that are discoverable spatially.
        </p>
        <figure>
          <img
            src="https://images.unsplash.com/photo-1516321318423-f06f85e504b3?auto=format&fit=crop&w=1200&q=80"
            alt="Desk with notes and diagrams"
          />
          <figcaption>
            Content can also mix text with media, not just links and paragraphs.
          </figcaption>
        </figure>
        <details open>
          <summary>Quick note</summary>
          <p>
            The iframe content now uses richer HTML so styling edge cases are easier to spot.
          </p>
        </details>
        <p>
          This branch can also expand into <a data-paper-id="media-stack">media stack</a>
          and <a data-paper-id="style-recipes">style recipes</a>.
        </p>
      </article>
    `,
    parentId: 'root',
    childIds: ['media-stack', 'style-recipes'],
  },
  {
    id: 'focus',
    title: 'Focused Reading',
    description: 'Readers expand only the concepts they need.',
    content: `
      <section>
        <h3>Focused reading</h3>
        <p>
          Focused reading reduces overload. It keeps the current context visible
          while allowing a local deep dive into one branch.
        </p>
        <p class="mini-note">
          Think of it as progressive disclosure for documents rather than menus.
        </p>
        <p>
          Related ideas: <a data-paper-id="reading-path">reading path</a> and
          <a data-paper-id="context-window">context window</a>.
        </p>
      </section>
    `,
    parentId: 'concept',
    childIds: ['reading-path', 'context-window'],
  },
  {
    id: 'sidebar',
    title: 'Sidebar Staging',
    description: 'Unplaced nodes can wait off-canvas.',
    content: `
      <section>
        <h3>Sidebar staging</h3>
        <p>
          The sidebar stores papers that are not currently attached to a room.
          They can be dragged into the tree when needed.
        </p>
        <ul>
          <li>Temporary parking for ideas</li>
          <li>Low-friction insertion point</li>
          <li>Useful during restructuring</li>
        </ul>
        <p>
          The staging area becomes more useful with <a data-paper-id="draft-basket">draft basket</a>
          and <a data-paper-id="reentry-flow">re-entry flow</a>.
        </p>
      </section>
    `,
    parentId: 'concept',
    childIds: ['draft-basket', 'reentry-flow'],
  },
  {
    id: 'core-state',
    title: 'Core State',
    description: 'Tree and expansion state stay explicit.',
    content: `
      <section>
        <h3>Core state</h3>
        <p>
          Tree structure, open children, focus, and access timestamps are modeled
          separately so reducers stay predictable.
        </p>
        <pre><code>paperMap + openChildren + focusedNodeId</code></pre>
        <p>
          Drill into <a data-paper-id="state-slices">state slices</a> or
          <a data-paper-id="reducer-commands">reducer commands</a>.
        </p>
      </section>
    `,
    parentId: 'architecture',
    childIds: ['state-slices', 'reducer-commands'],
  },
  {
    id: 'iframe-bridge',
    title: 'Iframe Bridge',
    description: 'HTML content lives in isolated iframes.',
    content: `
      <section>
        <h3>Iframe bridge</h3>
        <p>
          Each paper renders HTML in an iframe and reports link clicks plus height
          changes back to the parent through <code>postMessage</code>.
        </p>
        <p>
          The bridge keeps the host React tree in control while letting content remain
          expressive and self-contained.
        </p>
        <p>
          Important details are <a data-paper-id="message-channel">message channel</a>
          and <a data-paper-id="resize-sync">resize sync</a>.
        </p>
      </section>
    `,
    parentId: 'architecture',
    childIds: ['message-channel', 'resize-sync'],
  },
  {
    id: 'layout-engine',
    title: 'Layout Engine',
    description: 'Size is derived rather than stored.',
    content: `
      <section>
        <h3>Layout engine</h3>
        <p>
          Node size should come from derived layout calculations, not mutable
          authoritative state. That keeps space propagation coherent.
        </p>
        <hr />
        <p class="mini-note">
          Derived layout also makes recursive rendering easier to reason about.
        </p>
        <p>
          Follow <a data-paper-id="space-propagation">space propagation</a> or
          <a data-paper-id="split-strategy">split strategy</a>.
        </p>
      </section>
    `,
    parentId: 'architecture',
    childIds: ['space-propagation', 'split-strategy'],
  },
  {
    id: 'breadcrumbs',
    title: 'Breadcrumbs',
    description: 'Only the focused branch is shown.',
    content: `
      <section>
        <h3>Breadcrumbs</h3>
        <p>
          Breadcrumbs are built from the focused node back to root. Clicking an
          ancestor closes the branch below it and moves focus upward.
        </p>
        <p>
          Two useful details are <a data-paper-id="focus-chain">focus chain</a> and
          <a data-paper-id="ancestor-close">ancestor close</a>.
        </p>
      </section>
    `,
    parentId: 'interaction',
    childIds: ['focus-chain', 'ancestor-close'],
  },
  {
    id: 'drag-drop',
    title: 'Drag and Drop',
    description: 'Cards and open nodes can be repositioned.',
    content: `
      <section>
        <h3>Drag and drop</h3>
        <p>
          Closed cards can be opened or moved. Open nodes can also be dragged into
          a different room to change parentage.
        </p>
        <p>
          <kbd>Pointer down</kbd> on a chip starts the bridge event that the host
          uses to begin dragging.
        </p>
        <p>
          The interaction depends on <a data-paper-id="drag-gesture">drag gesture</a>
          and <a data-paper-id="drop-targeting">drop targeting</a>.
        </p>
      </section>
    `,
    parentId: 'interaction',
    childIds: ['drag-gesture', 'drop-targeting'],
  },
  {
    id: 'reading-path',
    title: 'Reading Path',
    description: 'Readers trace a selective route through the tree.',
    content: `
      <section>
        <h4>Reading path</h4>
        <p>
          A reader rarely traverses every child. Instead they open a route that matches
          their current question.
        </p>
        <p>
          That route is shaped by <a data-paper-id="entry-cues">entry cues</a>.
        </p>
      </section>
    `,
    parentId: 'focus',
    childIds: ['entry-cues'],
  },
  {
    id: 'context-window',
    title: 'Context Window',
    description: 'The parent paper remains visible while drilling down.',
    content: `
      <section>
        <h4>Context window</h4>
        <p>
          The parent acts like a stable frame so deeper content does not feel detached.
        </p>
      </section>
    `,
    parentId: 'focus',
    childIds: [],
  },
  {
    id: 'entry-cues',
    title: 'Entry Cues',
    description: 'Links should signal what kind of detail lies underneath.',
    content: `
      <section>
        <h4>Entry cues</h4>
        <p>
          Labels, visual chips, and nearby prose should help the reader predict what opens next.
        </p>
      </section>
    `,
    parentId: 'reading-path',
    childIds: [],
  },
  {
    id: 'draft-basket',
    title: 'Draft Basket',
    description: 'Temporary holding zone for papers not yet placed.',
    content: `
      <section>
        <h4>Draft basket</h4>
        <p>
          A staging shelf lets authors collect promising nodes before deciding where they belong.
        </p>
        <p>
          One refinement is <a data-paper-id="candidate-ranking">candidate ranking</a>.
        </p>
      </section>
    `,
    parentId: 'sidebar',
    childIds: ['candidate-ranking'],
  },
  {
    id: 'reentry-flow',
    title: 'Re-entry Flow',
    description: 'Returning a staged paper into the tree should stay lightweight.',
    content: `
      <section>
        <h4>Re-entry flow</h4>
        <p>
          Reinserted papers should preserve the sense that they were prepared, not discarded.
        </p>
      </section>
    `,
    parentId: 'sidebar',
    childIds: [],
  },
  {
    id: 'candidate-ranking',
    title: 'Candidate Ranking',
    description: 'Sidebar items can be ordered by likelihood of reuse.',
    content: `
      <section>
        <h4>Candidate ranking</h4>
        <p>
          Time, focus, and manual pinning can all influence which unplaced papers float to the top.
        </p>
      </section>
    `,
    parentId: 'draft-basket',
    childIds: [],
  },
  {
    id: 'state-slices',
    title: 'State Slices',
    description: 'Separate maps keep concerns explicit.',
    content: `
      <section>
        <h4>State slices</h4>
        <p>
          Expansion, importance, placement, and focus each change for different reasons.
        </p>
        <p>
          That separation helps when tuning <a data-paper-id="derived-selectors">derived selectors</a>.
        </p>
      </section>
    `,
    parentId: 'core-state',
    childIds: ['derived-selectors'],
  },
  {
    id: 'reducer-commands',
    title: 'Reducer Commands',
    description: 'Commands express the legal transitions in the tree.',
    content: `
      <section>
        <h4>Reducer commands</h4>
        <p>
          Open, close, move, and focus transitions are easier to inspect when modeled explicitly.
        </p>
      </section>
    `,
    parentId: 'core-state',
    childIds: [],
  },
  {
    id: 'derived-selectors',
    title: 'Derived Selectors',
    description: 'Selectors turn raw maps into view-facing structure.',
    content: `
      <section>
        <h4>Derived selectors</h4>
        <p>
          A selector layer can hide storage details while exposing a stable recursive view model.
        </p>
      </section>
    `,
    parentId: 'state-slices',
    childIds: [],
  },
  {
    id: 'message-channel',
    title: 'Message Channel',
    description: 'Iframe events are relayed through postMessage.',
    content: `
      <section>
        <h4>Message channel</h4>
        <p>
          The iframe emits structured events so the host can open nodes or begin drags.
        </p>
        <p>
          Filtering rules matter in <a data-paper-id="event-guard">event guard</a>.
        </p>
      </section>
    `,
    parentId: 'iframe-bridge',
    childIds: ['event-guard'],
  },
  {
    id: 'resize-sync',
    title: 'Resize Sync',
    description: 'Content height changes are observed and reported upward.',
    content: `
      <section>
        <h4>Resize sync</h4>
        <p>
          The room layout depends on timely content height reports from the iframe.
        </p>
      </section>
    `,
    parentId: 'iframe-bridge',
    childIds: [],
  },
  {
    id: 'event-guard',
    title: 'Event Guard',
    description: 'Only trusted message shapes should update host state.',
    content: `
      <section>
        <h4>Event guard</h4>
        <p>
          Runtime validation keeps malformed events from corrupting the interaction loop.
        </p>
      </section>
    `,
    parentId: 'message-channel',
    childIds: [],
  },
  {
    id: 'space-propagation',
    title: 'Space Propagation',
    description: 'Available space needs to cascade through recursive rooms.',
    content: `
      <section>
        <h4>Space propagation</h4>
        <p>
          Child rectangles depend on the dimensions left over after content and siblings are placed.
        </p>
        <p>
          A tricky case is <a data-paper-id="overflow-policy">overflow policy</a>.
        </p>
      </section>
    `,
    parentId: 'layout-engine',
    childIds: ['overflow-policy'],
  },
  {
    id: 'split-strategy',
    title: 'Split Strategy',
    description: 'Layout decides how to partition room space between content and children.',
    content: `
      <section>
        <h4>Split strategy</h4>
        <p>
          Some branches benefit from wide reading columns, while others want denser child panes.
        </p>
      </section>
    `,
    parentId: 'layout-engine',
    childIds: [],
  },
  {
    id: 'overflow-policy',
    title: 'Overflow Policy',
    description: 'When space is tight, scroll and clipping rules define the fallback.',
    content: `
      <section>
        <h4>Overflow policy</h4>
        <p>
          Derived layout works best when overflow behavior is predictable at every depth.
        </p>
      </section>
    `,
    parentId: 'space-propagation',
    childIds: [],
  },
  {
    id: 'focus-chain',
    title: 'Focus Chain',
    description: 'Breadcrumbs are reconstructed from the focused node upward.',
    content: `
      <section>
        <h4>Focus chain</h4>
        <p>
          Parent links let the UI rebuild the visible branch without storing a separate path array.
        </p>
      </section>
    `,
    parentId: 'breadcrumbs',
    childIds: [],
  },
  {
    id: 'ancestor-close',
    title: 'Ancestor Close',
    description: 'Selecting an ancestor collapses everything below it.',
    content: `
      <section>
        <h4>Ancestor close</h4>
        <p>
          This keeps the branch compact and reorients focus at the same time.
        </p>
      </section>
    `,
    parentId: 'breadcrumbs',
    childIds: [],
  },
  {
    id: 'drag-gesture',
    title: 'Drag Gesture',
    description: 'Dragging begins from iframe content and continues in the host.',
    content: `
      <section>
        <h4>Drag gesture</h4>
        <p>
          The system hands off pointer intent from the iframe to the outer React layer.
        </p>
        <p>
          Precision depends on <a data-paper-id="pointer-handoff">pointer handoff</a>.
        </p>
      </section>
    `,
    parentId: 'drag-drop',
    childIds: ['pointer-handoff'],
  },
  {
    id: 'drop-targeting',
    title: 'Drop Targeting',
    description: 'Insertion feedback should make the target room obvious.',
    content: `
      <section>
        <h4>Drop targeting</h4>
        <p>
          Good target feedback reduces accidental reparenting in dense recursive layouts.
        </p>
      </section>
    `,
    parentId: 'drag-drop',
    childIds: [],
  },
  {
    id: 'pointer-handoff',
    title: 'Pointer Handoff',
    description: 'Drag origin data is translated into host coordinates.',
    content: `
      <section>
        <h4>Pointer handoff</h4>
        <p>
          Once dragging starts, the host owns hit testing and room registration.
        </p>
      </section>
    `,
    parentId: 'drag-gesture',
    childIds: [],
  },
  {
    id: 'media-stack',
    title: 'Media Stack',
    description: 'Text, imagery, and annotations can coexist in one paper.',
    content: `
      <section>
        <h4>Media stack</h4>
        <p>
          Rich demo content is useful because it forces layout and iframe styling to handle variety.
        </p>
        <p>
          It pairs naturally with <a data-paper-id="caption-patterns">caption patterns</a>.
        </p>
      </section>
    `,
    parentId: 'notes',
    childIds: ['caption-patterns'],
  },
  {
    id: 'style-recipes',
    title: 'Style Recipes',
    description: 'Reusable content blocks help test CSS coverage.',
    content: `
      <section>
        <h4>Style recipes</h4>
        <p>
          Callouts, code blocks, tables, and details tags are all useful visual probes.
        </p>
      </section>
    `,
    parentId: 'notes',
    childIds: [],
  },
  {
    id: 'caption-patterns',
    title: 'Caption Patterns',
    description: 'Captions need spacing and contrast rules that fit the paper tone.',
    content: `
      <section>
        <h4>Caption patterns</h4>
        <p>
          Supporting text should feel attached to media without competing with the main paragraph flow.
        </p>
      </section>
    `,
    parentId: 'media-stack',
    childIds: [],
  },
];

export function buildDemoPaperMap(): PaperMap {
  return buildPaperMap(papers);
}
