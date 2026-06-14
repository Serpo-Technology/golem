<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Status,
    Search,
    Concepts,
    Audit,
    Observe,
    Teach,
    Edit,
    Mark,
    Delete,
  } from '../wailsjs/go/main/App';
  import { wire } from '../wailsjs/go/models';

  type Tab = 'search' | 'concepts' | 'audit' | 'author';
  let tab: Tab = 'concepts';

  let status: wire.StatusInfo | null = null;
  let error = '';

  let query = '';
  let results: wire.FragmentInfo[] = [];
  let searched = false;

  let concepts: wire.ConceptInfo[] = [];
  let trail: wire.AuditInfo[] = [];

  // Inline surgery state, one row at a time.
  let editingId: number | null = null;
  let editText = '';
  let confirmDeleteId: number | null = null;

  // Authoring forms.
  let obsChannel = 'tell';
  let obsText = '';
  let teachCue = '';
  let teachCmd = '';
  let authored = '';

  function fail(e: any) {
    error = e && e.toString ? e.toString() : String(e);
  }

  async function refresh() {
    error = '';
    try {
      status = await Status();
    } catch (e) {
      fail(e);
    }
  }

  async function loadConcepts() {
    try {
      concepts = await Concepts();
    } catch (e) {
      fail(e);
    }
  }

  async function loadAudit() {
    try {
      trail = await Audit();
    } catch (e) {
      fail(e);
    }
  }

  async function runSearch() {
    error = '';
    searched = true;
    try {
      results = await Search(query);
    } catch (e) {
      fail(e);
    }
  }

  function startEdit(f: wire.FragmentInfo) {
    editingId = f.id;
    editText = f.content;
    confirmDeleteId = null;
  }

  async function saveEdit(f: wire.FragmentInfo) {
    try {
      await Edit(f.id, editText);
      editingId = null;
      await runSearch();
      await refresh();
    } catch (e) {
      fail(e);
    }
  }

  async function doMark(f: wire.FragmentInfo) {
    try {
      await Mark(f.id);
      await runSearch();
    } catch (e) {
      fail(e);
    }
  }

  async function doDelete(f: wire.FragmentInfo) {
    try {
      await Delete(f.id);
      confirmDeleteId = null;
      await runSearch();
      await refresh();
    } catch (e) {
      fail(e);
    }
  }

  async function doObserve() {
    if (!obsText) return;
    try {
      await Observe(obsChannel, obsText);
      authored = `observed on ${obsChannel}.`;
      obsText = '';
      await refresh();
    } catch (e) {
      fail(e);
    }
  }

  async function doTeach() {
    if (!teachCue || !teachCmd) return;
    try {
      await Teach(teachCue, teachCmd);
      authored = `grounded "${teachCue}".`;
      teachCue = '';
      teachCmd = '';
    } catch (e) {
      fail(e);
    }
  }

  function uptime(sec: number): string {
    const s = Math.floor(sec);
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m`;
    return `${s}s`;
  }

  function when(unix: number): string {
    return new Date(unix * 1000).toLocaleString();
  }

  onMount(async () => {
    await refresh();
    await loadConcepts();
    await loadAudit();
  });
</script>

<main>
  <header>
    <div class="brand">
      <h1>GME</h1>
      <span class="sub">GOLEM memory</span>
    </div>

    {#if status}
      <div class="vitals">
        <span>pid {status.pid}</span>
        <span>up {uptime(status.uptime_sec)}</span>
        <span><b>{status.fragments}</b> fragments</span>
        <span><b>{status.concepts}</b> concepts</span>
        <span><b>{status.words}</b> words</span>
        <span><b>{status.sleeps}</b> nights</span>
      </div>
      <div class="chem" title="dopamine / noradrenaline / serotonin / oxytocin">
        <i class="dot da" style="opacity:{0.25 + 0.75 * status.chem.Dopamine}"></i>
        <i class="dot na" style="opacity:{0.25 + 0.75 * status.chem.Noradrenaline}"></i>
        <i class="dot se" style="opacity:{0.25 + 0.75 * status.chem.Serotonin}"></i>
        <i class="dot ox" style="opacity:{0.25 + 0.75 * status.chem.Oxytocin}"></i>
      </div>
    {/if}

    <button class="refresh" on:click={refresh}>refresh</button>
  </header>

  {#if status && status.goal}
    <div class="goal"><span>goal</span> {status.goal}</div>
  {/if}

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <nav>
    <button class:active={tab === 'search'} on:click={() => (tab = 'search')}>Search</button>
    <button class:active={tab === 'concepts'} on:click={() => (tab = 'concepts')}>Concepts</button>
    <button class:active={tab === 'audit'} on:click={() => (tab = 'audit')}>Audit</button>
    <button class:active={tab === 'author'} on:click={() => (tab = 'author')}>Author</button>
  </nav>

  <section>
    {#if tab === 'search'}
      <form class="searchbar" on:submit|preventDefault={runSearch}>
        <input placeholder="recall a cue..." bind:value={query} />
        <button type="submit">recall</button>
      </form>
      {#if results.length}
        <ul class="rows">
          {#each results as f (f.id)}
            <li>
              <span class="id">#{f.id}</span>
              <span class="chip">{f.kind || 'episodic'}</span>
              <span class="chip ch">{f.channel}</span>
              {#if editingId === f.id}
                <input class="editbox" bind:value={editText} />
                <button class="mini ok" on:click={() => saveEdit(f)}>save</button>
                <button class="mini" on:click={() => (editingId = null)}>cancel</button>
              {:else}
                <span class="content">{f.content}</span>
                {#if f.edited}<span class="edited">edited</span>{/if}
                <span class="rowtools">
                  <button class="mini" on:click={() => startEdit(f)}>edit</button>
                  <button class="mini" on:click={() => doMark(f)}>mark</button>
                  {#if confirmDeleteId === f.id}
                    <button class="mini danger" on:click={() => doDelete(f)}>confirm</button>
                    <button class="mini" on:click={() => (confirmDeleteId = null)}>no</button>
                  {:else}
                    <button class="mini" on:click={() => (confirmDeleteId = f.id)}>delete</button>
                  {/if}
                </span>
              {/if}
            </li>
          {/each}
        </ul>
      {:else if searched}
        <p class="empty">nothing comes to mind.</p>
      {:else}
        <p class="empty">recall the moments a cue brings to mind, then shape them in place.</p>
      {/if}
    {:else if tab === 'concepts'}
      {#if concepts.length}
        <ul class="rows">
          {#each concepts as c}
            <li>
              <span class="id">#{c.id}</span>
              <span class="gloss">{c.gloss}</span>
              {#if c.names && c.names.length}
                <span class="names">
                  {#each c.names as n}<em>{n}</em>{/each}
                </span>
              {/if}
              <span class="grounded">grounded {c.grounded}</span>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="empty">no concepts born yet; they distil from recurring shapes during sleep.</p>
      {/if}
    {:else if tab === 'audit'}
      {#if trail.length}
        <ul class="rows trail">
          {#each trail as e}
            <li>
              <span class="ts">{when(e.at_unix)}</span>
              <span class="cue">{e.cue}</span>
              <span class="arrow">-&gt;</span>
              <span class="action">{e.action}</span>
              <span class="outcome">{e.outcome}</span>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="empty">the trail is empty.</p>
      {/if}
    {:else if tab === 'author'}
      <div class="author">
        <p class="hint">
          Authoring grounds through GOLEM's own teaching, so what you add is earned the way lived
          experience is.
        </p>
        <form on:submit|preventDefault={doObserve}>
          <label>Observe a moment</label>
          <div class="formrow">
            <input class="chan" placeholder="channel" bind:value={obsChannel} />
            <input class="grow" placeholder="what happened..." bind:value={obsText} />
            <button type="submit">observe</button>
          </div>
        </form>
        <form on:submit|preventDefault={doTeach}>
          <label>Teach a cue</label>
          <div class="formrow">
            <input class="chan" placeholder="cue" bind:value={teachCue} />
            <input class="grow" placeholder="command" bind:value={teachCmd} />
            <button type="submit">teach</button>
          </div>
        </form>
        {#if authored}<p class="ok-msg">{authored}</p>{/if}
      </div>
    {/if}
  </section>
</main>

<style>
  main {
    height: 100vh;
    display: flex;
    flex-direction: column;
    color: #d7dae0;
  }

  header {
    display: flex;
    align-items: center;
    gap: 18px;
    padding: 14px 18px;
    border-bottom: 1px solid #2a2e36;
    background: #16181c;
  }

  .brand {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .brand h1 {
    margin: 0;
    font-size: 18px;
    letter-spacing: 2px;
    color: #f0f2f5;
  }
  .brand .sub {
    font-size: 12px;
    color: #6b727e;
  }

  .vitals {
    display: flex;
    gap: 14px;
    font-size: 12px;
    color: #9aa1ac;
  }
  .vitals b {
    color: #e6e8ec;
  }

  .chem {
    display: flex;
    gap: 5px;
    margin-left: auto;
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    display: inline-block;
  }
  .dot.da {
    background: #e2b341;
  }
  .dot.na {
    background: #e0564b;
  }
  .dot.se {
    background: #4ca5e0;
  }
  .dot.ox {
    background: #59c08a;
  }

  .refresh {
    background: #232730;
    color: #c4c9d2;
    border: 1px solid #343a45;
    border-radius: 6px;
    padding: 6px 12px;
    cursor: pointer;
    font-size: 12px;
  }
  .refresh:hover {
    background: #2c313b;
  }

  .goal {
    padding: 8px 18px;
    font-size: 13px;
    color: #c9b98a;
    background: #1d1b15;
    border-bottom: 1px solid #2a2820;
  }
  .goal span {
    color: #7d745a;
    margin-right: 8px;
    text-transform: uppercase;
    font-size: 11px;
    letter-spacing: 1px;
  }

  .error {
    padding: 10px 18px;
    background: #2a1715;
    color: #e89a90;
    font-size: 13px;
    border-bottom: 1px solid #46221d;
  }

  nav {
    display: flex;
    gap: 4px;
    padding: 10px 18px 0;
    border-bottom: 1px solid #2a2e36;
  }
  nav button {
    background: none;
    border: none;
    color: #8a909b;
    padding: 8px 14px;
    cursor: pointer;
    font-size: 13px;
    border-bottom: 2px solid transparent;
  }
  nav button.active {
    color: #f0f2f5;
    border-bottom-color: #4ca5e0;
  }

  section {
    flex: 1;
    overflow-y: auto;
    padding: 14px 18px;
  }

  .searchbar {
    display: flex;
    gap: 8px;
    margin-bottom: 14px;
  }
  .searchbar input {
    flex: 1;
    background: #1b1e24;
    border: 1px solid #343a45;
    border-radius: 6px;
    color: #e6e8ec;
    padding: 8px 12px;
    font-size: 14px;
  }
  .searchbar input:focus {
    outline: none;
    border-color: #4ca5e0;
  }
  .searchbar button {
    background: #2b6da3;
    color: #fff;
    border: none;
    border-radius: 6px;
    padding: 8px 16px;
    cursor: pointer;
  }

  ul.rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  ul.rows li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 6px;
    border-bottom: 1px solid #23262d;
    font-size: 13px;
  }
  ul.rows li:hover {
    background: #1a1d23;
  }

  .id {
    color: #5f6772;
    font-variant-numeric: tabular-nums;
    min-width: 56px;
  }
  .chip {
    background: #232a33;
    color: #8fb4d6;
    border-radius: 4px;
    padding: 1px 7px;
    font-size: 11px;
  }
  .chip.ch {
    color: #b59ad6;
  }
  .content {
    color: #d7dae0;
  }

  .edited {
    background: #2a2415;
    color: #d2b85a;
    border-radius: 4px;
    padding: 1px 7px;
    font-size: 11px;
  }

  .rowtools {
    margin-left: auto;
    display: flex;
    gap: 4px;
    opacity: 0;
  }
  ul.rows li:hover .rowtools {
    opacity: 1;
  }
  .mini {
    background: #232730;
    color: #aeb4be;
    border: 1px solid #343a45;
    border-radius: 5px;
    padding: 2px 9px;
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
  .editbox {
    flex: 1;
    background: #1b1e24;
    border: 1px solid #4ca5e0;
    border-radius: 5px;
    color: #e6e8ec;
    padding: 4px 8px;
    font-size: 13px;
  }
  .editbox:focus {
    outline: none;
  }

  .gloss {
    color: #e6e8ec;
  }
  .names {
    display: flex;
    gap: 5px;
    flex-wrap: wrap;
  }
  .names em {
    font-style: normal;
    background: #1f2630;
    color: #84c9a3;
    border-radius: 4px;
    padding: 1px 7px;
    font-size: 11px;
  }
  .grounded {
    margin-left: auto;
    color: #6b727e;
    font-size: 12px;
  }

  .trail li {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
  }
  .ts {
    color: #5f6772;
    min-width: 150px;
  }
  .cue {
    color: #8fb4d6;
  }
  .arrow {
    color: #565d68;
  }
  .action {
    color: #c9b98a;
  }
  .outcome {
    color: #9aa1ac;
  }

  .author {
    max-width: 720px;
  }
  .author .hint {
    color: #7d8590;
    font-size: 13px;
    margin: 0 0 16px;
  }
  .author label {
    display: block;
    color: #9aa1ac;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 6px;
  }
  .author form {
    margin-bottom: 18px;
  }
  .formrow {
    display: flex;
    gap: 8px;
  }
  .formrow input {
    background: #1b1e24;
    border: 1px solid #343a45;
    border-radius: 6px;
    color: #e6e8ec;
    padding: 8px 12px;
    font-size: 14px;
  }
  .formrow input:focus {
    outline: none;
    border-color: #4ca5e0;
  }
  .formrow input.chan {
    width: 130px;
  }
  .formrow input.grow {
    flex: 1;
  }
  .formrow button {
    background: #2b6da3;
    color: #fff;
    border: none;
    border-radius: 6px;
    padding: 8px 16px;
    cursor: pointer;
  }
  .ok-msg {
    color: #84c9a3;
    font-size: 13px;
  }

  .empty {
    color: #6b727e;
    font-size: 13px;
    padding: 20px 6px;
  }
</style>
