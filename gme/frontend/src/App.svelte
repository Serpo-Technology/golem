<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    Status, Concepts, Cognition, Health, Audit, Search,
    Tell, See, Observe, Trust, Enroll,
    Retune, Runtime, Voice, Sleep, Forget,
    Edit, Mark, Delete, Connect, ConnInfo,
  } from '../wailsjs/go/main/App';
  import { EventsOn } from '../wailsjs/runtime/runtime';
  import { wire, client } from '../wailsjs/go/models';
  import Graph from './Graph.svelte';

  type Panel = 'chat' | 'senses' | 'memory' | 'trail' | 'mind' | 'identity' | 'controls' | 'settings';
  let panel: Panel = 'chat';

  let status: wire.StatusInfo | null = null;
  let conn: client.Info | null = null;
  let streamUp = false;
  let error = '';
  function fail(e: any) { error = e?.message ?? String(e); }
  function clearErr() { error = ''; }

  // --- live event stream ---
  type Ev = { at: number; ev: string; cue?: string; outcome?: string; tpl?: string; channel?: string };
  let activity: Ev[] = [];
  let lastReply = '';
  let lastReplyAt = 0;

  // --- chat ---
  type Turn = { who: 'me' | 'golem'; text: string; unprompted?: boolean };
  let turns: Turn[] = [];
  let chatInput = '';
  let chatLang = '';
  let voiceOut = false;
  let listening = false;
  let recog: any = null;

  async function refresh() {
    try { status = await Status(); clearErr(); } catch (e) { fail(e); }
  }
  async function loadConn() { try { conn = await ConnInfo(); } catch (e) { fail(e); } }

  function pushTurn(t: Turn) { turns = [...turns.slice(-200), t]; }
  function speak(text: string) {
    if (!voiceOut || !text) return;
    try {
      const u = new SpeechSynthesisUtterance(text);
      const name = status?.voice_name;
      if (name) { const v = speechSynthesis.getVoices().find(x => x.name === name); if (v) u.voice = v; }
      speechSynthesis.speak(u);
    } catch { /* no speech synthesis here */ }
  }

  async function send() {
    const text = chatInput.trim();
    if (!text) return;
    pushTurn({ who: 'me', text });
    chatInput = '';
    try {
      const resp = await Tell(text, chatLang);
      const say = (resp?.say ?? '').trim();
      if (say) { pushTurn({ who: 'golem', text: say }); lastReply = say; lastReplyAt = Date.now(); speak(say); }
      clearErr();
      refresh();
    } catch (e) { fail(e); }
  }

  function onEvent(ev: Ev) {
    activity = [ev, ...activity.slice(0, 300)];
    if (ev.ev === 'say' && ev.tpl) {
      const fresh = ev.tpl.trim();
      if (fresh && !(fresh === lastReply && Date.now() - lastReplyAt < 2500)) {
        pushTurn({ who: 'golem', text: fresh, unprompted: true });
        speak(fresh);
      }
    }
  }

  // --- voice in ---
  function toggleListen() {
    const SR = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SR) { fail('speech recognition is not available in this window'); return; }
    if (listening) { recog?.stop(); listening = false; return; }
    recog = new SR();
    recog.lang = chatLang === 'tr' ? 'tr-TR' : 'en-US';
    recog.interimResults = false;
    recog.onresult = (e: any) => { chatInput = e.results[0][0].transcript; listening = false; send(); };
    recog.onerror = () => { listening = false; };
    recog.onend = () => { listening = false; };
    recog.start();
    listening = true;
  }

  // --- senses ---
  let obsChannel = 'reading';
  let obsText = '';
  let seeCaption = '';
  let seeThumb = '';
  async function doObserve() {
    if (!obsText.trim()) return;
    try { const r = await Observe(obsChannel, obsText); obsText = ''; clearErr(); refresh(); seeCaption = (r?.say ?? '') || 'filed.'; } catch (e) { fail(e); }
  }
  async function onImage(e: Event) {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    const buf = new Uint8Array(await file.arrayBuffer());
    let bin = ''; for (const b of buf) bin += String.fromCharCode(b);
    const b64 = btoa(bin);
    seeThumb = `data:${file.type};base64,${b64}`;
    try { const r = await See(b64, file.type, chatLang); seeCaption = r?.say ?? ''; clearErr(); } catch (err) { fail(err); }
  }

  // --- memory ---
  let query = '';
  let results: wire.FragmentInfo[] = [];
  let searched = false;
  let concepts: wire.ConceptInfo[] = [];
  let memView: 'search' | 'concepts' | 'graph' = 'search';
  let editingId: number | null = null;
  let editText = '';
  let confirmDeleteId: number | null = null;
  async function runSearch() {
    try { results = (await Search(query)) ?? []; searched = true; clearErr(); } catch (e) { fail(e); }
  }
  async function loadConcepts() { try { concepts = (await Concepts()) ?? []; clearErr(); } catch (e) { fail(e); } }
  function startEdit(f: wire.FragmentInfo) { editingId = f.id; editText = f.content; }
  async function saveEdit(f: wire.FragmentInfo) {
    try { await Edit(f.id, editText); f.content = editText; f.edited = true; editingId = null; results = results; } catch (e) { fail(e); }
  }
  async function doMark(f: wire.FragmentInfo) { try { await Mark(f.id); f.edited = true; results = results; } catch (e) { fail(e); } }
  async function doDelete(f: wire.FragmentInfo) {
    try { await Delete(f.id); results = results.filter(r => r.id !== f.id); confirmDeleteId = null; } catch (e) { fail(e); }
  }

  // --- trail ---
  let trail: wire.AuditInfo[] = [];
  async function loadAudit() { try { trail = (await Audit()) ?? []; clearErr(); } catch (e) { fail(e); } }

  // --- mind ---
  let cog: wire.CognitionInfo | null = null;
  let health: wire.Health | null = null;
  async function loadMind() {
    try { cog = await Cognition(); health = await Health(); clearErr(); } catch (e) { fail(e); }
  }

  // --- identity ---
  let trustSource = 'llm', trustLevel = 0.6, trustSay = '';
  let enrollSource = 'llm', enrollKey = '', enrollSay = '';
  async function doTrust() { try { trustSay = await Trust(trustSource, trustLevel, true); clearErr(); } catch (e) { fail(e); } }
  async function readTrust() { try { trustSay = await Trust(trustSource, 0, false); clearErr(); } catch (e) { fail(e); } }
  async function doEnroll() { try { enrollSay = await Enroll(enrollSource, enrollKey); clearErr(); } catch (e) { fail(e); } }
  async function doRevoke() { try { enrollSay = await Enroll(enrollSource, ''); clearErr(); } catch (e) { fail(e); } }

  // --- controls ---
  let ctlSay = '';
  let retuneFile = 'internal/daemon/experiment.go', retuneConst = 'experimentConf', retuneVal = '0.6';
  let voiceName = '';
  let forgetQuery = '';
  async function runtime(arg: string) { try { ctlSay = await Runtime(arg); refresh(); clearErr(); } catch (e) { fail(e); } }
  async function voice(arg: string) { try { const v = await Voice(arg, voiceName); ctlSay = v?.say ?? ''; refresh(); clearErr(); } catch (e) { fail(e); } }
  async function doSleep() { try { await Sleep(); ctlSay = 'consolidated once.'; refresh(); clearErr(); } catch (e) { fail(e); } }
  async function doRetune() { try { ctlSay = await Retune(retuneFile, retuneConst, retuneVal); clearErr(); } catch (e) { fail(e); } }
  async function doForget() { try { ctlSay = await Forget(forgetQuery); forgetQuery = ''; refresh(); clearErr(); } catch (e) { fail(e); } }

  // --- settings ---
  let sNet = 'tcp', sAddr = '127.0.0.1:7777', sKey = '';
  async function doConnect() {
    try { conn = await Connect(sNet, sAddr, sKey); clearErr(); refresh(); } catch (e) { fail(e); }
  }

  function pick(p: Panel) {
    panel = p;
    if (p === 'memory' && memView === 'concepts') loadConcepts();
    if (p === 'trail') loadAudit();
    if (p === 'mind') loadMind();
  }

  let timer: any;
  onMount(async () => {
    await loadConn();
    await refresh();
    EventsOn('golem:event', onEvent);
    EventsOn('golem:stream', (s: any) => { streamUp = !!s?.connected; });
    timer = setInterval(refresh, 4000);
  });
  onDestroy(() => clearInterval(timer));

  function uptime(sec: number): string {
    if (!sec) return '0s';
    const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = Math.floor(sec % 60);
    return h ? `${h}h${m}m` : m ? `${m}m${s}s` : `${s}s`;
  }
  function when(unix: number): string {
    return new Date(unix * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }
  function evTime(ms: number): string {
    return new Date(ms).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }
</script>

<main>
  <header>
    <div class="brand">
      <h1>Golem Toolkit</h1>
      <span class="conn" class:live={streamUp}>
        {conn ? `${conn.net} ${conn.addr}` : '...'}{conn && !conn.signed ? ' (no key)' : ''}
        <i class="dot" class:on={streamUp}></i>
      </span>
    </div>
    {#if status}
      <div class="vitals">
        <span>up {uptime(status.uptime_sec)}</span>
        <span>{status.fragments} frag</span>
        <span>{status.concepts} concept</span>
        <span>{status.sleeps} sleep</span>
      </div>
      <div class="chem" title="dopamine / noradrenaline / serotonin / oxytocin">
        <i style="background:#e2b341;opacity:{status.chem.Dopamine}"></i>
        <i style="background:#e0564b;opacity:{status.chem.Noradrenaline}"></i>
        <i style="background:#4ca5e0;opacity:{status.chem.Serotonin}"></i>
        <i style="background:#59c08a;opacity:{status.chem.Oxytocin}"></i>
      </div>
    {/if}
    <button class="refresh" on:click={refresh}>refresh</button>
  </header>

  {#if status?.goal}<div class="goal"><span>goal</span> {status.goal}</div>{/if}
  {#if error}<div class="error" role="alert"><span>{error}</span><button class="dismiss" on:click={clearErr}>dismiss</button></div>{/if}

  <div class="body">
    <nav>
      <button class:active={panel==='chat'} on:click={() => pick('chat')}>Chat</button>
      <button class:active={panel==='senses'} on:click={() => pick('senses')}>Senses</button>
      <button class:active={panel==='memory'} on:click={() => pick('memory')}>Memory</button>
      <button class:active={panel==='trail'} on:click={() => pick('trail')}>Trail</button>
      <button class:active={panel==='mind'} on:click={() => pick('mind')}>Mind</button>
      <button class:active={panel==='identity'} on:click={() => pick('identity')}>Identity</button>
      <button class:active={panel==='controls'} on:click={() => pick('controls')}>Controls</button>
      <button class:active={panel==='settings'} on:click={() => pick('settings')}>Settings</button>
    </nav>

    <section>
      {#if panel === 'chat'}
        <div class="chat">
          <div class="transcript">
            {#each turns as t}
              <div class="turn {t.who}" class:unprompted={t.unprompted}>
                <span class="who">{t.who === 'me' ? 'you' : 'golem'}{t.unprompted ? ' (aloud)' : ''}</span>
                <span class="text">{t.text}</span>
              </div>
            {/each}
            {#if turns.length === 0}<div class="hint">Say something. GOLEM answers in what it understands, and speaks its own thoughts here too.</div>{/if}
          </div>
          <form class="composer" on:submit|preventDefault={send}>
            <select bind:value={chatLang} title="language"><option value="">en</option><option value="tr">tr</option><option value="la">la</option></select>
            <input placeholder="tell GOLEM something" bind:value={chatInput} />
            <button type="button" class:active={listening} on:click={toggleListen} title="speak">{listening ? '...' : 'mic'}</button>
            <button type="button" class:active={voiceOut} on:click={() => voiceOut = !voiceOut} title="speak replies aloud">voice</button>
            <button type="submit">send</button>
          </form>
        </div>

      {:else if panel === 'senses'}
        <div class="stack">
          <div class="card">
            <h3>Observe</h3>
            <p class="muted">File a grounded moment on a channel, the way a sense feeds one in.</p>
            <div class="row">
              <input class="chan" bind:value={obsChannel} placeholder="channel" />
              <input bind:value={obsText} placeholder="a red door" on:keydown={(e) => e.key === 'Enter' && doObserve()} />
              <button on:click={doObserve}>observe</button>
            </div>
          </div>
          <div class="card">
            <h3>Show an image</h3>
            <p class="muted">GOLEM measures the pixels itself and answers in its own words.</p>
            <input type="file" accept="image/*" on:change={onImage} />
            {#if seeThumb}<img class="thumb" src={seeThumb} alt="shown" />{/if}
            {#if seeCaption}<div class="say">{seeCaption}</div>{/if}
          </div>
        </div>

      {:else if panel === 'memory'}
        <div class="subnav">
          <button class:active={memView==='search'} on:click={() => memView='search'}>Search</button>
          <button class:active={memView==='concepts'} on:click={() => { memView='concepts'; loadConcepts(); }}>Concepts</button>
          <button class:active={memView==='graph'} on:click={() => memView='graph'}>Graph</button>
        </div>
        {#if memView === 'search'}
          <form class="searchbar" on:submit|preventDefault={runSearch}>
            <input placeholder="recall by a cue" bind:value={query} />
            <button type="submit">recall</button>
          </form>
          <ul class="rows">
            {#each results as f (f.id)}
              <li>
                {#if editingId === f.id}
                  <textarea bind:value={editText} rows="2"></textarea>
                  <button class="mini ok" on:click={() => saveEdit(f)}>save</button>
                  <button class="mini" on:click={() => (editingId = null)}>cancel</button>
                {:else}
                  <span class="frag">{f.content}</span>
                  <span class="chip">{f.channel}</span>
                  {#if f.edited}<span class="edited">edited</span>{/if}
                  <span class="tools">
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
            {#if searched && results.length === 0}<li class="hint">nothing comes to mind.</li>{/if}
          </ul>
        {:else if memView === 'concepts'}
          <ul class="rows">
            {#each concepts as c (c.id)}
              <li>
                <span class="frag">{c.gloss}</span>
                <span class="chip cat">{c.category}</span>
                {#each c.names ?? [] as n}<span class="name">{n}</span>{/each}
                <span class="grounded">{c.grounded}x</span>
              </li>
            {/each}
          </ul>
        {:else}
          <Graph />
        {/if}

      {:else if panel === 'trail'}
        <div class="split">
          <div class="col">
            <h3>Decision trail</h3>
            <ul class="mono">
              {#each trail as a}
                <li><span class="t">{when(a.at_unix)}</span> <span class="cue">{a.cue}</span> <span class="act">{a.action}</span> <span class="out">{a.outcome}</span></li>
              {/each}
            </ul>
          </div>
          <div class="col">
            <h3>Live activity <i class="dot" class:on={streamUp}></i></h3>
            <ul class="mono">
              {#each activity as e}
                <li><span class="t">{evTime(e.at)}</span> <span class="ev">{e.ev}</span> <span class="out">{e.tpl || e.cue || e.outcome || ''}</span></li>
              {/each}
            </ul>
          </div>
        </div>

      {:else if panel === 'mind'}
        <div class="stack">
          {#if cog}
            <div class="card">
              <h3>Cognition</h3>
              <div class="grid">
                <div><b>{cog.concepts}</b> concepts</div>
                <div><b>{cog.features}</b> features</div>
                <div><b>{cog.above_floor}</b> pairs resemble</div>
                <div>floor <b>{cog.floor?.toFixed?.(2)}</b></div>
              </div>
              {#if cog.universal?.length}<p class="muted">universal: {cog.universal.join(', ')}</p>{/if}
            </div>
          {/if}
          {#if health}
            <div class="card">
              <h3>Health</h3>
              {#if !health.duplicates?.length && !health.promiscuous?.length}
                <p class="muted">clean: no split meanings, no promiscuous words.</p>
              {:else}
                {#if health.duplicates?.length}<p>{health.duplicates.length} split meaning(s)</p>{/if}
                {#if health.promiscuous?.length}<p>{health.promiscuous.length} promiscuous word(s)</p>{/if}
              {/if}
            </div>
          {/if}
        </div>

      {:else if panel === 'identity'}
        <div class="stack">
          <div class="card">
            <h3>Trust</h3>
            <p class="muted">How far GOLEM believes a source. Setting it needs your signature.</p>
            <div class="row">
              <input bind:value={trustSource} placeholder="source" />
              <input type="number" min="0" max="1" step="0.05" bind:value={trustLevel} />
              <button on:click={doTrust}>set</button>
              <button on:click={readTrust}>read</button>
            </div>
            {#if trustSay}<div class="say">{trustSay}</div>{/if}
          </div>
          <div class="card">
            <h3>Enroll a peer key</h3>
            <p class="muted">Register a peer's public key so it can prove its own identity. Empty revokes.</p>
            <div class="row">
              <input bind:value={enrollSource} placeholder="source" />
              <input bind:value={enrollKey} placeholder="base64 public key" />
              <button on:click={doEnroll}>enroll</button>
              <button on:click={doRevoke}>revoke</button>
            </div>
            {#if enrollSay}<div class="say">{enrollSay}</div>{/if}
          </div>
        </div>

      {:else if panel === 'controls'}
        <div class="stack">
          <div class="card">
            <h3>Runtime & voice</h3>
            <div class="row">
              <span class="muted">runtime</span>
              <button on:click={() => runtime('on')}>on</button>
              <button on:click={() => runtime('off')}>off</button>
              <span class="muted">voice</span>
              <button on:click={() => voice('on')}>on</button>
              <button on:click={() => voice('off')}>off</button>
              <input class="chan" bind:value={voiceName} placeholder="voice name" />
              <button on:click={() => voice('use')}>use</button>
            </div>
          </div>
          <div class="card">
            <h3>Consolidate & forget</h3>
            <div class="row">
              <button on:click={doSleep}>sleep once</button>
              <input bind:value={forgetQuery} placeholder="forget what a phrase names" />
              <button on:click={doForget}>forget</button>
            </div>
          </div>
          <div class="card">
            <h3>Retune a constant</h3>
            <p class="muted">Build and test a candidate with one tuning constant changed. Needs your signature; the deploy stays yours.</p>
            <div class="row">
              <input bind:value={retuneFile} placeholder="file" />
              <input class="chan" bind:value={retuneConst} placeholder="constant" />
              <input class="chan" bind:value={retuneVal} placeholder="value" />
              <button on:click={doRetune}>retune</button>
            </div>
          </div>
          {#if ctlSay}<div class="say">{ctlSay}</div>{/if}
        </div>

      {:else if panel === 'settings'}
        <div class="stack">
          <div class="card">
            <h3>Connection</h3>
            <p class="muted">Point the toolkit at a brain. The container's brain listens on TCP and needs your signing key; a brain beside you on the unix socket needs none.</p>
            <div class="row">
              <select bind:value={sNet}><option value="tcp">tcp</option><option value="unix">unix</option></select>
              <input bind:value={sAddr} placeholder="127.0.0.1:7777 or socket path" />
            </div>
            <div class="row">
              <input bind:value={sKey} placeholder="signing key: a file path, or empty for GOLEM_PRIVATE_KEY" />
              <button on:click={doConnect}>connect</button>
            </div>
            {#if conn}<div class="say">now on {conn.net} {conn.addr}{conn.signed ? ', signed' : ', unsigned'}</div>{/if}
          </div>
        </div>
      {/if}
    </section>
  </div>
</main>

<style>
  :global(html), :global(body) { margin: 0; height: 100%; background: #121418; }
  main { display: flex; flex-direction: column; height: 100vh;
    color: #d7dae0; font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased; }

  header { display: flex; align-items: center; gap: 18px; padding: 10px 16px;
    background: #16181c; border-bottom: 1px solid #2a2e36; }
  .brand { display: flex; align-items: baseline; gap: 12px; }
  .brand h1 { font-size: 16px; margin: 0; letter-spacing: .3px; color: #e6e8ec; }
  .conn { font-size: 11px; color: #6b727e; display: inline-flex; align-items: center; gap: 6px; }
  .conn.live { color: #9aa1ac; }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: #5f6772; display: inline-block; }
  .dot.on { background: #59c08a; }
  .vitals { display: flex; gap: 12px; font-size: 12px; color: #9aa1ac; margin-left: auto; }
  .chem { display: flex; gap: 4px; }
  .chem i { width: 9px; height: 9px; border-radius: 50%; display: inline-block; }
  .refresh { background: #232730; border: 1px solid #343a45; border-radius: 6px; color: #c4c9d2; padding: 5px 11px; cursor: pointer; font-size: 12px; }
  .refresh:hover { background: #2c313b; }

  .goal { padding: 6px 16px; background: #1d1b15; color: #d2b85a; font-size: 12px; }
  .goal span { color: #8a7a3f; text-transform: uppercase; letter-spacing: 1px; margin-right: 8px; font-size: 10px; }
  .error { padding: 8px 16px; background: #2a1715; color: #e0796f; font-size: 12px; display: flex; align-items: center; gap: 12px; }
  .error span { flex: 1; }
  .dismiss { background: none; border: 1px solid #5a2b26; border-radius: 5px; color: #e0796f; padding: 2px 9px; font-size: 11px; }

  .body { display: flex; flex: 1; min-height: 0; }
  nav { display: flex; flex-direction: column; gap: 2px; padding: 12px 8px; width: 108px; border-right: 1px solid #23262d; background: #14161a; }
  nav button { background: none; border: none; color: #8a919c; text-align: left; padding: 7px 12px; border-radius: 6px; cursor: pointer; font-size: 13px; }
  nav button:hover { background: #1a1d23; color: #cfd4db; }
  nav button.active { background: #202634; color: #e6e8ec; }

  section { flex: 1; min-width: 0; overflow-y: auto; padding: 16px 18px; }

  .hint { color: #6b727e; font-size: 13px; padding: 8px 0; }
  .muted { color: #8a919c; font-size: 12px; margin: 2px 0 8px; }
  .say { margin-top: 8px; padding: 8px 10px; background: #16211b; color: #a7d8bf; border-radius: 6px; font-size: 13px; }

  .chat { display: flex; flex-direction: column; height: 100%; }
  .transcript { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 8px; padding-bottom: 10px; }
  .turn { display: flex; flex-direction: column; gap: 2px; max-width: 78%; }
  .turn.me { align-self: flex-end; align-items: flex-end; }
  .turn .who { font-size: 10px; text-transform: uppercase; letter-spacing: 1px; color: #6b727e; }
  .turn .text { padding: 8px 12px; border-radius: 10px; background: #1b1e24; }
  .turn.me .text { background: #223247; color: #dfe8f2; }
  .turn.unprompted .text { background: #1a2019; color: #bcd6c6; font-style: italic; }
  .composer { display: flex; gap: 6px; padding-top: 10px; border-top: 1px solid #23262d; }
  .composer input { flex: 1; }
  .composer select { width: 56px; }

  input, select, textarea { background: #1b1e24; border: 1px solid #343a45; border-radius: 6px; color: #e6e8ec; padding: 8px 11px; font-size: 14px; font-family: inherit; }
  input:focus, select:focus, textarea:focus { outline: none; border-color: #4ca5e0; }
  input.chan { max-width: 130px; }

  button { background: #232730; border: 1px solid #343a45; border-radius: 6px; color: #c4c9d2; padding: 7px 13px; cursor: pointer; font-size: 13px; }
  button:hover { background: #2c313b; }
  button.active { background: #2b6da3; color: #fff; border-color: #2b6da3; }

  .stack { display: flex; flex-direction: column; gap: 14px; }
  .card { background: #16181c; border: 1px solid #23262d; border-radius: 8px; padding: 14px 16px; }
  .card h3 { margin: 0 0 2px; font-size: 13px; color: #e6e8ec; }
  .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  .row input { flex: 1; min-width: 120px; }
  .grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; font-size: 13px; color: #b9c0cb; }
  .grid b { color: #e6e8ec; }
  .thumb { max-width: 200px; max-height: 160px; border-radius: 6px; margin: 10px 0; display: block; border: 1px solid #23262d; }

  .subnav { display: flex; gap: 4px; margin-bottom: 12px; }
  .subnav button { padding: 5px 12px; font-size: 12px; }
  .subnav button.active { background: #202634; color: #e6e8ec; }

  .searchbar { display: flex; gap: 8px; margin-bottom: 12px; }
  .searchbar input { flex: 1; }

  ul.rows { list-style: none; margin: 0; padding: 0; }
  ul.rows li { display: flex; gap: 10px; align-items: center; padding: 8px 6px; border-bottom: 1px solid #23262d; }
  ul.rows li:hover { background: #1a1d23; }
  ul.rows textarea { flex: 1; }
  .frag { flex: 1; }
  .chip { background: #232a33; color: #8fb4d6; border-radius: 4px; padding: 1px 7px; font-size: 11px; }
  .chip.cat { color: #b59ad6; }
  .name { background: #1f2630; color: #84c9a3; border-radius: 4px; padding: 1px 7px; font-size: 11px; }
  .grounded { color: #6b727e; font-size: 11px; }
  .edited { background: #2a2415; color: #d2b85a; border-radius: 4px; padding: 1px 7px; font-size: 11px; }
  .tools { display: flex; gap: 4px; opacity: 0; }
  ul.rows li:hover .tools { opacity: 1; }
  .mini { padding: 2px 9px; font-size: 11px; border-radius: 5px; }
  .mini.ok { background: #2b6da3; color: #fff; border-color: #2b6da3; }
  .mini.danger { background: #7a2820; color: #fff; border-color: #7a2820; }

  .split { display: flex; gap: 16px; height: 100%; }
  .split .col { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .split h3 { font-size: 12px; color: #9aa1ac; text-transform: uppercase; letter-spacing: 1px; margin: 0 0 8px; }
  ul.mono { list-style: none; margin: 0; padding: 0; overflow-y: auto; font: 12px/1.7 ui-monospace, SFMono-Regular, Menlo, monospace; }
  ul.mono li { padding: 1px 0; border-bottom: 1px solid #191c22; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .mono .t { color: #5f6772; }
  .mono .cue, .mono .ev { color: #8fb4d6; }
  .mono .act { color: #d2b85a; }
  .mono .out { color: #9aa1ac; }
</style>
