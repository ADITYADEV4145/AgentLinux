/* SPDX-License-Identifier: Apache-2.0 */
'use strict';
// The capability exists only in memory and same-origin navigation fragments.
// No provider token, desktop content or credentials are written to browser storage.
const fragment = location.hash.slice(1).match(/^([a-f0-9]{64})(?::(how|pricing))?$/);
let capability = fragment ? fragment[1] : '';
const initialSection = fragment?.[2];
history.replaceState(null, '', location.pathname);
const $ = id => document.getElementById(id);
const isDashboard = location.pathname === '/dashboard';
for (const a of document.querySelectorAll('a[href="/#pricing"], a[href="/#how"]')) {
 const section = a.hash.slice(1);
 a.href = isDashboard && capability ? '/#' + capability + ':' + section : '/#' + section;
}
if (initialSection) document.getElementById(initialSection)?.scrollIntoView();
$('landing').hidden = isDashboard;
$('dashboard').hidden = !isDashboard;
for (const a of document.querySelectorAll('a[href="/"]')) a.href = '/' + (capability ? '#' + capability : '');
for (const a of document.querySelectorAll('.console-link')) {
  a.href = '/dashboard' + (capability ? '#' + capability : '');
}
function node(tag, text, cls) {
  const e = document.createElement(tag); if (text !== undefined) e.textContent = text; if (cls) e.className = cls; return e;
}
function notice(text, error = false) { $('notice').textContent = text; $('notice').className = 'notice' + (error ? ' error' : ''); }
async function api(path, method = 'GET', body) {
  const controller = new AbortController(); const timer = setTimeout(() => controller.abort(), 10000);
  try {
    const r = await fetch('/api/' + path, {method, signal: controller.signal, cache: 'no-store', credentials: 'omit',
      headers: {'Authorization': 'Bearer ' + capability, 'Content-Type': 'application/json'}, ...(body ? {body: JSON.stringify(body)} : {})});
    if (r.status === 401) throw new Error('Reopen the private URL printed by agent-linux dashboard. The console capability is missing or expired.');
    const value = await r.json();
    if (!r.ok) throw new Error((value.error || 'UNAVAILABLE') + ': ' + (value.message || 'Operation unavailable'));
    return value;
  } catch (e) {
    if (e.name === 'AbortError' || e instanceof TypeError) throw new Error('Connection unavailable; a mutation may have succeeded. Refresh machine status before trying again.');
    throw e;
  } finally { clearTimeout(timer); }
}
const money = micros => typeof micros === 'number' ? '$' + (micros / 1e6).toFixed(4) : 'Unavailable';
async function pricing() {
  try {
    const r = await fetch('/pricing.json', {cache: 'no-store'}); const p = await r.json();
    const cards = [{name: 'Local', hourly: 0, local: true}, ...p.plans];
    $('prices').replaceChildren(...cards.map((plan, i) => {
      const card = node('article', undefined, 'price' + (i === 1 ? ' featured' : ''));
      card.append(node('span', plan.local ? 'YOURS, ALWAYS' : (plan.id === 'builder' ? 'PLANNED SIZE · SELECTED PRICE' : 'SELECTED PRICE · NOT ON SALE'), 'eyebrow'));
      card.append(node('h3', plan.name));
      const amount = node('div', '$' + (plan.hourly === 0 ? '0' : plan.hourly.toFixed(2)), 'amount');
      amount.append(node('span', plan.local ? ' / software usage' : ' / machine-hour')); card.append(amount);
      const list = node('ul');
      for (const text of plan.local ? ['Runs on your Apple silicon Mac', 'Persistent Linux desktop', 'MCP + human control', 'Your hardware and energy costs apply'] :
        [plan.cpus + ' vCPU · ' + plan.memoryGiB + ' GiB RAM', plan.diskGiB + ' GiB ephemeral disk target', 'Separate machine per task', 'Pay as you go or choose a plan']) list.append(node('li', text));
      card.append(list);
      if (plan.subscription) {
        const sub = plan.subscription;
        const options = node('div', undefined, 'plan-options');
        const monthly = node('div');
        monthly.append(node('strong', '$' + sub.monthlyUSD + ' / month'), node('span', sub.includedHoursPerMonth + ' machine-hours each month'));
        const annual = node('div');
        annual.append(node('strong', '$' + sub.annualUSD + ' / year'), node('span', sub.includedHoursPerMonth + ' hours/month · billed yearly'));
        annual.append(node('small', 'Save $' + (sub.monthlyUSD * 12 - sub.annualUSD) + ' vs monthly · $' + (sub.annualUSD / 12).toFixed(2) + '/mo equivalent'));
        options.append(monthly, annual);
        card.append(options);
      }
      const a = node('a', plan.local ? 'Start on your Mac ↗' : 'View cloud development →', 'button' + (i === 1 ? ' primary' : ''));
      a.href = plan.local ? 'https://github.com/ADITYADEV4145/AgentLinux#quick-start' : '/dashboard' + (capability ? '#' + capability : ''); card.append(a);
      card.append(node('p', plan.local ? 'No hosted account required.' : 'Cloud sales and subscriptions are not active.', 'fine'));
      return card;
    }));
  } catch { $('prices').textContent = 'Pricing unavailable. Local software usage is free; no paid cloud offer is active.'; }
}
$('copy').addEventListener('click', async () => {
  try { await navigator.clipboard.writeText('agent-linux'); $('copy').textContent = 'Copied ✓'; }
  catch { $('copy').textContent = 'Select to copy'; }
});
pricing();
let config = null, fleet = null, loading = false, mutating = false;
let pendingCreate = null;
function enableCreate() {
  const count = fleet && !config?.local ? fleet.sessions.filter(s => s.state !== 'stopped').length : 0;
  $('new-machine').disabled = !config?.connected || !fleet || mutating || (!config.local && count >= fleet.limits.maxPerOwner);
}
function renderFleet(value) {
  fleet = value;
  const active = value.sessions.filter(s => s.state !== 'stopped').length;
  $('active-count').textContent = String(active);
  $('concurrency').textContent = String(value.limits.maxPerOwner);
  $('max-duration').textContent = value.limits.maxSeconds / 60 + ' min';
  $('duration').max = String(value.limits.maxSeconds);
  if (!value.sessions.length) {
    const empty = node('div', undefined, 'empty'); empty.append(node('span', '▣', 'empty-icon'), node('h3', 'A fresh machine for your next idea.'), node('p', 'Create a desktop session. Its backend sets the resource profile, rate, idle timeout and lifetime.'));
    $('fleet').replaceChildren(empty);
  } else {
    $('fleet').replaceChildren(...value.sessions.map(s => {
      const row = node('article', undefined, 'machine-row'); const name = node('div');
      name.append(node('strong', 'linux-' + s.id.slice(0, 10)), node('small', s.provider));
      if (s.simulated) name.append(node('span', 'SIMULATED · NO LINUX VM', 'simulated'));
      const state = node('div'); state.append(node('span', '● ' + s.state.toUpperCase(), 'machine-state'));
      state.append(node('small', s.state === 'stopped' ? 'Guest data discarded' : 'Expires ' + new Date(s.expires).toLocaleTimeString()));
      state.append(node('small', s.billing ? 'Usage: ' + money(s.billing.netMicros) + ' (inspect ledger below)' : 'Runtime estimate: ' + s.estimatedRuntimeSeconds + 's'));
      const actions = node('div', undefined, 'row-actions');
      const view = node('button', 'Open ↗'); view.disabled = s.state !== 'ready';
      view.addEventListener('click', () => openViewer(s.id, view));
      const stop = node('button', 'Stop'); stop.disabled = s.state === 'stopped' || s.state === 'stopping';
      stop.addEventListener('click', () => stopMachine(s, stop));
      actions.append(view, stop); row.append(name, state, actions); return row;
    }));
  }
  const simulated = value.sessions.some(s => s.simulated);
  notice(simulated ? 'SIMULATED DEVELOPMENT: the default provider runs no Linux VM. Sessions and limits are real control-plane records.' : 'Development connection active. Real provider support is experimental; opening this console creates no compute.');
  enableCreate();
}
function renderBilling(b) {
  $('billing-raw').textContent = JSON.stringify(b, null, 2);
  const fields = [['Mode', b.mode || 'sandbox'], ['Local plan', 'Free'], ['Payment activation', 'Test/sandbox only']];
  if (b.mode === 'disabled') fields.push(['Cloud rates', 'Not configured — no finalized retail prices']);
  else {
    // Keep the exact response visible rather than guessing field names or totals.
    fields.push(['Cloud ledger', 'Available below; estimates subject to reconciliation']);
    if (b.account?.policy) fields.push(['Spending cap', money(b.account.policy.spendingCapMicros)], ['Trial remaining', money(b.trialRemainingMicros)], ['Finalized usage', money(b.finalizedNetMicros)], ['Active estimate', money(b.estimatedActiveNetMicros)], ['Reserved usage', money(b.reservedActiveNetMicros)]);
    if (b.warning) fields.push(['Warning', b.warning]);
    if (Array.isArray(b.rates)) for (const rate of b.rates) fields.push([rate.id + ' · ' + rate.cpus + ' CPU / ' + rate.memoryGiB + ' GiB', money(rate.microsPerHour) + '/hour']);
  }
  $('billing-details').replaceChildren(...fields.map(([k,v]) => {const e=node('div'); e.append(node('dt',k),node('dd',v)); return e;}));
}
async function refresh() {
  if (loading || document.hidden || !config?.connected) return;
  loading = true;
  try { if (config.local) renderLocal(await api('local/machines')); else renderFleet(await api('sessions')); }
  catch (e) { notice(e.message, true); $('new-machine').disabled = true; }
  finally { loading = false; }
}
async function openViewer(id, button) {
  // Open synchronously so browser popup blocking doesn't swallow an async navigation.
  const tab = window.open('about:blank', '_blank'); if (tab) tab.opener = null;
  button.disabled = true;
  try {
    const result = await api((config.local ? 'local/machines/' : 'sessions/') + id + '/view', 'POST', {}); const u = new URL(result.url);
    if (u.protocol !== 'http:' || u.hostname !== '127.0.0.1' || !/^[A-Za-z0-9_-]{43}$/.test(u.hash.slice(1))) throw new Error('Invalid private viewer URL');
    if (tab) tab.location.replace(u.href); else notice('Popup blocked. Allow popups for this local console, then open the viewer again.', true);
  } catch (e) { if (tab) tab.close(); notice(e.message, true); }
  finally { button.disabled = false; }
}
async function stopMachine(s, button) {
  if (!window.confirm(config.local ? 'Stop ' + s.name + '? Saved guest files persist; unsaved application work may be lost.' : 'Stop linux-' + s.id.slice(0, 10) + '? This terminates only this selected session and discards all its guest files.')) return;
  button.disabled = true;
  try { await api((config.local ? 'local/machines/' : 'sessions/') + s.id + '/stop', 'POST', {confirm: true}); await refresh(); }
  catch (e) { notice(e.message, true); button.disabled = false; }
}
$('new-machine').addEventListener('click', () => $('create-dialog').showModal());
$('create-form').addEventListener('submit', async e => {
  if (e.submitter?.value !== 'create') return;
  e.preventDefault();
  if (config.local) { await createLocal(); return; }
  const seconds = Number($('duration').value);
  if (!Number.isInteger(seconds) || seconds < 30 || seconds > fleet.limits.maxSeconds || mutating) return;
  if (pendingCreate && pendingCreate.seconds !== seconds) { notice('A previous create result is uncertain. Refresh and reconcile it before changing the request.', true); $('create-dialog').close(); return; }
  if (!pendingCreate) pendingCreate = {requestId: Array.from(crypto.getRandomValues(new Uint8Array(16)), n => n.toString(16).padStart(2, '0')).join(''), seconds};
  mutating = true; enableCreate(); $('confirm-create').disabled = true;
  try {
    await api('sessions', 'POST', pendingCreate); pendingCreate = null; $('create-dialog').close(); await refresh();
  } catch (e) { $('create-dialog').close(); notice(e.message + ' The same create intent retains its idempotency key in this tab.', true); }
  finally { mutating = false; $('confirm-create').disabled = false; enableCreate(); }
});
$('refresh').addEventListener('click', async () => { await refresh(); try {if (!config?.local) renderBilling(await api('billing'));} catch(e) {notice(e.message,true);} });
async function connect() {
  if (!isDashboard) return;
  if (!capability) {notice('Open the private URL printed by agent-linux dashboard, then use Open console. Refreshing or bookmarking this page does not retain its credential.', true); return;}
  try {
    config = await api('config');
    if (!config.connected) { notice('No cloud identity connected. Local Linux is available through agent-linux. To manage development cloud sessions, restart dashboard with --cloud-config FILE.'); return; }
    if (config.local) configureLocal();
    await refresh(); if (!config.local) renderBilling(await api('billing'));
    setInterval(refresh, 3000);
  } catch (e) {notice(e.message,true);}
}
connect();

function configureLocal() {
 $('local-create').hidden=false; $('local-create').disabled=false;
 $('duration').disabled=true; $('duration').hidden=true;
 document.querySelector('label[for="duration"]').hidden=true;
 document.querySelector('#create-form > .muted').textContent='Create a persistent Linux computer on this Mac. Aggregate CPU/RAM budgets and disk-space checks apply. NAT networking can reach host/LAN services.';
 document.querySelector('#create-form > .fine').textContent='No automatic restart or reset. Closing the viewer leaves Linux running; stopping preserves saved guest data.';
 document.querySelector('.sidebar > strong').textContent='This Mac';
 document.querySelector('.console-help p').textContent='Open a ready machine and choose Take control. F2 hands input back in Kitty. Closing the viewer/dashboard leaves Linux running. Stop explicitly; saved files persist.';
 document.querySelector('#usage > .muted').textContent='Local software usage is free. Budgets govern managed guest allocations, not every VM or process on your Mac. No payment account is needed.';
 document.querySelector('#usage > details').hidden=true;
}
function renderLocal(result) {
 const value=result.fleet; fleet={sessions:value.machines};
 const running=value.machines.filter(m=>m.allocated).length;
 $('active-count').textContent=String(running);
 $('concurrency').parentElement.querySelector('span').textContent='MANAGED CPU BUDGET';
 $('concurrency').textContent=value.budget.cpus+' vCPU';
 $('max-duration').parentElement.querySelector('span').textContent='MANAGED RAM BUDGET';
 $('max-duration').textContent=value.budget.memoryGiB+' GiB';
 $('fleet').replaceChildren(...value.machines.map(m=>{
  const row=node('article',undefined,'machine-row'),name=node('div'),state=node('div'),actions=node('div',undefined,'row-actions');
  const job=result.jobs[m.id];const phase=job?.state==='starting'?(job.operation||'starting'):m.state;
  name.append(node('strong',m.name),node('small',m.id.slice(0,10)+' · local VZ'));
  state.append(node('span','● '+phase.toUpperCase(),'machine-state'),node('small',m.cpus+' vCPU · '+m.memoryGiB+' GiB · persistent disk'));
  if(job?.state==='error') state.append(node('small',job.error));
  const view=node('button','Open ↗');view.disabled=phase!=='ready';view.addEventListener('click',()=>openViewer(m.id,view));
  const start=node('button','Start');start.disabled=!['stopped','not_created','error'].includes(phase);start.addEventListener('click',async()=>{start.disabled=true;try{await api('local/machines/'+m.id+'/start','POST',{});await refresh();}catch(e){notice(e.message,true);}});
  const stop=node('button','Stop');stop.disabled=!['ready','error','unavailable'].includes(phase);stop.addEventListener('click',()=>stopMachine(m,stop));
  const recovery=node('button','Recovery');recovery.disabled=job?.state==='starting';recovery.addEventListener('click',()=>openRecovery(m));actions.append(view,start,stop,recovery);row.append(name,state,actions);return row;
 }));
 if (!value.machines.length) {const empty=node('div',undefined,'empty');empty.append(node('h3','Your first Linux workspace.'),node('p','Create a named machine here, or run agent-linux start --machine work. Existing custom namespaces can be explicitly registered through the CLI.'));$('fleet').append(empty);}
 const pairs=[['Local software','Free'],['Managed allocation limits',value.budget.cpus+' vCPU / '+value.budget.memoryGiB+' GiB'],['Persistence','Saved files survive stop and restart'],['Scope',value.scope]];
 $('billing-details').replaceChildren(...pairs.map(([k,v])=>{const e=node('div');e.append(node('dt',k),node('dd',v));return e;}));
 notice('Local catalog connected. Readiness comes from the guest handshake. Viewers capture only on demand; no cloud or billing is involved.');enableCreate();
}
async function createLocal(){
 if(mutating)return;mutating=true;enableCreate();$('confirm-create').disabled=true;
 try{await api('local/machines','POST',{name:$('local-name').value,cpus:Number($('local-cpus').value),memoryGiB:Number($('local-memory').value),diskGiB:Number($('local-disk').value)});$('create-dialog').close();await refresh();}
 catch(e){notice(e.message+' Refresh the inventory before retrying; creation is never replayed automatically.',true);$('create-dialog').close();}
 finally{mutating=false;$('confirm-create').disabled=false;enableCreate();}
}

let recoveryMachine=null;
async function openRecovery(machine){
 try{const result=await api('local/machines/'+machine.id+'/checkpoints');recoveryMachine=machine;$('recovery-title').textContent=machine.name+' · recovery';const nodes=result.checkpoints.map(c=>node('p',c.name+' · '+c.status+' · '+new Date(c.createdAt).toLocaleString()));if(!nodes.length)nodes.push(node('p','No disk checkpoints yet.'));if(result.recovery)nodes.push(node('p','Pending '+result.recovery.kind+': '+result.recovery.phase+'. Rollback stays until a verified guest boot.'));$('checkpoint-list').replaceChildren(...nodes);$('recovery-confirm').checked=false;updateRecoveryWarning();$('recovery-dialog').showModal();}catch(e){notice(e.message,true);}
}
function updateRecoveryWarning(){
 const action=$('recovery-action').value;
 const warning={create:'Create a stopped-disk checkpoint. Maximum three per machine; old checkpoints are never silently deleted.',restore:'Replaces saved guest disk/configuration. Changes since the checkpoint will be lost. Rollback is retained until a fresh guest handshake. Start Linux explicitly afterward.',delete:'Permanently deletes the selected checkpoint. This does not reset the running computer.',recover:'Resume finishes an interrupted file installation. Rollback uses the preserved pre-operation disk. VM must be stopped; boot explicitly afterward.', 'desktop-restart':'Restarts the graphical session in the running guest. Unsaved application work can be lost. This does not restart the VM.', 'helper-upgrade':'Creates a stopped-disk checkpoint, stages the bundled helper-only upgrade, then boots and verifies it. Saved projects/identity persist. A failure retains rollback data.'};
 $('recovery-warning').textContent=warning[action];
 const named=['create','restore','delete'].includes(action);$('checkpoint-name').required=named;$('checkpoint-name').hidden=!named;$('checkpoint-name').disabled=!named;document.querySelector('label[for="checkpoint-name"]').hidden=!named;
 $('recovery-mode').hidden=action!=='recover';$('recovery-mode').disabled=action!=='recover';document.querySelector('label[for="recovery-mode"]').hidden=action!=='recover';
 $('recovery-confirm').required=action!=='create';$('recovery-confirm').closest('label').hidden=action==='create';
}
$('recovery-action').addEventListener('change',updateRecoveryWarning);
$('recovery-form').addEventListener('submit',async event=>{
 if(event.submitter?.value!=='apply')return;event.preventDefault();if(!recoveryMachine||mutating)return;
 const action=$('recovery-action').value;mutating=true;
 try{await api('local/machines/'+recoveryMachine.id+'/recovery','POST',{action,name:$('checkpoint-name').value,mode:$('recovery-mode').value,confirm:$('recovery-confirm').checked});$('recovery-dialog').close();await refresh();}catch(e){notice(e.message+' Inspect current status before retrying.',true);$('recovery-dialog').close();}finally{mutating=false;enableCreate();}
});
