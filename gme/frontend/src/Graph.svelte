<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import cytoscape from 'cytoscape';
  import { Dump, Edit, Mark, Delete } from '../wailsjs/go/main/App';
  import { wire } from '../wailsjs/go/models';

  const NODE_CAP = 600;
  const EDGE_CAP = 2500;

  // Which kinds are drawn. Episodic moments and the grammar scaffold are the bulk
  // and the noise, so the semantic core shows first and the rest toggles on.
  const GROUPS = ['concept', 'word', 'feature', 'episodic', 'grammar'];
  let enabled = new Set(['concept', 'word']);

  let container: HTMLDivElement;
  let cy: any = null;
  let graph: wire.Graph | null = null;
  let error = '';

  let shown = 0;
  let total = 0;
  let edgeShown = 0;

  let selected: wire.FragmentInfo | null = null;
  let editText = '';
  let confirmingDelete = false;

  function kindGroup(kind: string): string {
    if (kind === 'grammar' || kind === 'construction' || kind === 'operation') return 'grammar';
    if (!kind) return 'episodic';
    return kind;
  }

  function clip(s: string, n: number): string {
    return s.length > n ? s.slice(0, n) + '...' : s;
  }

  const STYLE: any = [
    {
      selector: 'node',
      style: {
        label: 'data(label)',
        'font-size': 7,
        color: '#9aa1ac',
        'background-color': '#5f6772',
        width: 9,
        height: 9,
        'text-valign': 'center',
        'text-halign': 'right',
        'text-margin-x': 3,
      },
    },
    { selector: 'node[group = "concept"]', style: { 'background-color': '#59c08a', width: 18, height: 18, 'font-size': 9 } },
    { selector: 'node[group = "word"]', style: { 'background-color': '#e2b341', width: 13, height: 13 } },
    { selector: 'node[group = "feature"]', style: { 'background-color': '#b59ad6' } },
    { selector: 'node[group = "grammar"]', style: { 'background-color': '#4ca5e0' } },
    { selector: 'node[?edited]', style: { 'border-color': '#d2b85a', 'border-width': 2 } },
    { selector: 'node:selected', style: { 'border-color': '#ffffff', 'border-width': 2 } },
    { selector: 'edge', style: { width: 1, 'line-color': '#2c313b', 'curve-style': 'haystack', opacity: 0.55 } },
    { selector: 'edge[rel = "names"]', style: { 'line-color': '#3a5a44' } },
    { selector: 'edge[rel = "grounds"]', style: { 'line-color': '#4a3a5a' } },
    { selector: 'edge[rel = "feature"]', style: { 'line-color': '#46405a' } },
  ];

  function elements() {
    if (!graph) return [];
    const all = (graph.fragments || []).filter((f) => enabled.has(kindGroup(f.kind)));
    total = all.length;
    const nodes = [...all].sort((a, b) => (b.weight || 0) - (a.weight || 0)).slice(0, NODE_CAP);
    shown = nodes.length;
    const ids = new Set(nodes.map((n) => n.id));
    const edges = (graph.links || [])
      .filter((l) => ids.has(l.from) && ids.has(l.to))
      .slice(0, EDGE_CAP);
    edgeShown = edges.length;
    return [
      ...nodes.map((f) => ({
        data: { id: String(f.id), label: clip(f.content, 22), group: kindGroup(f.kind), edited: !!f.edited },
      })),
      ...edges.map((l) => ({
        data: { id: `e${l.from}_${l.to}_${l.rel || 't'}`, source: String(l.from), target: String(l.to), rel: l.rel || 'temporal' },
      })),
    ];
  }

  function layoutFor(n: number): any {
    // Force layout reads well up to a few hundred nodes; beyond that a concentric
    // ring stays instant instead of churning.
    return n > 280
      ? { name: 'concentric', concentric: (n: any) => n.degree(), levelWidth: () => 4, padding: 20 }
      : { name: 'cose', animate: false, idealEdgeLength: 55, padding: 20 };
  }

  function render() {
    if (!container) return;
    const els = elements();
    if (cy) cy.destroy();
    cy = cytoscape({ container, elements: els, style: STYLE, wheelSensitivity: 0.2 });
    cy.layout(layoutFor(shown)).run();
    cy.on('tap', 'node', (e: any) => selectNode(Number(e.target.id())));
    cy.on('tap', (e: any) => {
      if (e.target === cy) selected = null;
    });
    cy.resize();
  }

  function selectNode(id: number) {
    const f = (graph?.fragments || []).find((x) => x.id === id) || null;
    selected = f;
    editText = f ? f.content : '';
    confirmingDelete = false;
  }

  function toggle(group: string) {
    if (enabled.has(group)) enabled.delete(group);
    else enabled.add(group);
    enabled = new Set(enabled);
    render();
  }

  async function reload() {
    try {
      graph = await Dump();
      render();
      if (selected) selectNode(selected.id);
    } catch (e) {
      error = e && (e as any).toString ? (e as any).toString() : String(e);
    }
  }

  async function saveEdit() {
    if (!selected) return;
    try {
      await Edit(selected.id, editText);
      await reload();
    } catch (e) {
      error = String(e);
    }
  }
  async function doMark() {
    if (!selected) return;
    try {
      await Mark(selected.id);
      await reload();
    } catch (e) {
      error = String(e);
    }
  }
  async function doDelete() {
    if (!selected) return;
    const id = selected.id;
    try {
      await Delete(id);
      selected = null;
      await reload();
    } catch (e) {
      error = String(e);
    }
  }

  onMount(async () => {
    try {
      graph = await Dump();
      await tick();
      render();
    } catch (e) {
      error = e && (e as any).toString ? (e as any).toString() : String(e);
    }
  });

  onDestroy(() => {
    if (cy) cy.destroy();
  });
</script>

<div class="graphwrap">
  <div class="toolbar">
    {#each GROUPS as g}
      <button class="chip" class:on={enabled.has(g)} on:click={() => toggle(g)}>{g}</button>
    {/each}
    <span class="count">
      {shown}{#if total > shown} of {total}{/if} nodes, {edgeShown} edges
    </span>
    <button class="relayout" on:click={render}>relayout</button>
  </div>

  {#if error}
    <div class="gerror">{error}</div>
  {/if}

  <div class="canvas" bind:this={container}></div>

  {#if selected}
    <aside class="inspector">
      <div class="ihead">
        <span class="iid">#{selected.id}</span>
        <span class="ikind">{selected.kind || 'episodic'}</span>
        <button class="x" on:click={() => (selected = null)}>close</button>
      </div>
      <div class="imeta">
        channel {selected.channel} &middot; weight {selected.weight.toFixed(2)}
        {#if selected.edited}&middot; <span class="ed">edited</span>{/if}
      </div>
      <textarea class="iedit" bind:value={editText} rows="3"></textarea>
      <div class="iactions">
        <button class="mini ok" on:click={saveEdit}>save edit</button>
        <button class="mini" on:click={doMark}>mark</button>
        {#if confirmingDelete}
          <button class="mini danger" on:click={doDelete}>confirm</button>
          <button class="mini" on:click={() => (confirmingDelete = false)}>no</button>
        {:else}
          <button class="mini" on:click={() => (confirmingDelete = true)}>delete</button>
        {/if}
      </div>
    </aside>
  {/if}
</div>

<style>
  .graphwrap {
    position: relative;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding-bottom: 10px;
  }
  .chip {
    background: #1b1e24;
    color: #6b727e;
    border: 1px solid #343a45;
    border-radius: 12px;
    padding: 3px 11px;
    cursor: pointer;
    font-size: 11px;
    text-transform: capitalize;
  }
  .chip.on {
    background: #232a33;
    color: #cfd4db;
    border-color: #4ca5e0;
  }
  .count {
    color: #6b727e;
    font-size: 12px;
    margin-left: 8px;
  }
  .relayout {
    margin-left: auto;
    background: #232730;
    color: #c4c9d2;
    border: 1px solid #343a45;
    border-radius: 6px;
    padding: 4px 11px;
    cursor: pointer;
    font-size: 12px;
  }

  .gerror {
    color: #e89a90;
    font-size: 12px;
    padding-bottom: 8px;
  }

  .canvas {
    flex: 1;
    min-height: 0;
    background: #15171b;
    border: 1px solid #23262d;
    border-radius: 8px;
  }

  .inspector {
    position: absolute;
    top: 44px;
    right: 0;
    width: 300px;
    background: #1b1e24;
    border: 1px solid #343a45;
    border-radius: 8px;
    padding: 12px;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.4);
  }
  .ihead {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .iid {
    color: #5f6772;
    font-variant-numeric: tabular-nums;
  }
  .ikind {
    background: #232a33;
    color: #8fb4d6;
    border-radius: 4px;
    padding: 1px 7px;
    font-size: 11px;
  }
  .x {
    margin-left: auto;
    background: none;
    border: none;
    color: #6b727e;
    cursor: pointer;
    font-size: 12px;
  }
  .imeta {
    color: #7d8590;
    font-size: 12px;
    margin: 8px 0;
  }
  .imeta .ed {
    color: #d2b85a;
  }
  .iedit {
    width: 100%;
    background: #15171b;
    border: 1px solid #343a45;
    border-radius: 6px;
    color: #e6e8ec;
    padding: 8px;
    font-size: 13px;
    font-family: inherit;
    resize: vertical;
    box-sizing: border-box;
  }
  .iedit:focus {
    outline: none;
    border-color: #4ca5e0;
  }
  .iactions {
    display: flex;
    gap: 5px;
    margin-top: 10px;
  }
  .mini {
    background: #232730;
    color: #aeb4be;
    border: 1px solid #343a45;
    border-radius: 5px;
    padding: 3px 10px;
    cursor: pointer;
    font-size: 11px;
  }
  .mini:hover {
    background: #2c313b;
  }
  .mini.ok {
    background: #2b6da3;
    color: #fff;
    border-color: #2b6da3;
  }
  .mini.danger {
    background: #7a2820;
    color: #fff;
    border-color: #7a2820;
  }
</style>
