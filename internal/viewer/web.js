'use strict';
// The fragment capability never enters an HTTP URL/log or persistent browser storage.
let token=location.hash.slice(1);history.replaceState(null,'','/');
const image=document.getElementById('desktop'),state=document.getElementById('state'),empty=document.getElementById('empty'),button=document.getElementById('pause');
let paused=false,etag='',objectURL='',last='',frameId='',generation='',owned=false,interactive=false,inputBusy=false;
const metrics=window.agentLinuxViewerMetrics={startedAt:performance.now(),imageLoads:0,requestMilliseconds:[],imageAgeMilliseconds:[]};
image.onload=()=>{metrics.imageLoads++;};
function unavailable(text){state.textContent=text;image.style.display='none';empty.style.display='block';empty.textContent=text;etag='';if(objectURL){URL.revokeObjectURL(objectURL);objectURL='';}image.removeAttribute('src');}
button.addEventListener('click',()=>{paused=!paused;button.textContent=paused?'Resume':'Pause';state.textContent=paused?'Paused — last image retained; VM keeps running':'Reconnecting…';});
async function tick(){
 if(!token){unavailable('Open the complete private viewer URL printed by agent-linux view.');return;}
 if(paused||document.hidden){setTimeout(tick,1000);return;}
 const requestStart=performance.now();
 const controller=new AbortController(),deadline=setTimeout(()=>controller.abort(),10000);
 try{
  const headers={'Authorization':'Bearer '+token};if(etag)headers['If-None-Match']=etag;
  const result=await fetch('/api/frame',{headers,cache:'no-store',signal:controller.signal});
  metrics.requestMilliseconds.push(performance.now()-requestStart);if(metrics.requestMilliseconds.length>256)metrics.requestMilliseconds.shift();
  const observed=Date.parse(result.headers.get('X-Capture-Time')||'');if(Number.isFinite(observed)){metrics.imageAgeMilliseconds.push(Math.max(0,Date.now()-observed));if(metrics.imageAgeMilliseconds.length>256)metrics.imageAgeMilliseconds.shift();}
  if(paused||document.hidden)return;
  if(result.status===200){
   const blob=await result.blob(),url=URL.createObjectURL(blob),old=objectURL;objectURL=url;
   image.src=url;if(old)URL.revokeObjectURL(old);image.style.display='block';empty.style.display='none';etag=result.headers.get('ETag')||'';
  }else if(result.status!==304){
   let code='UNAVAILABLE';try{code=(await result.json()).code||code;}catch{}
   unavailable(code+' — waiting for guest; use agent-linux status for diagnostics');
  }
  if(result.status===200||result.status===304){frameId=result.headers.get('X-Frame-ID')||'';const nextGeneration=result.headers.get('X-Desktop-Generation')||'';if(generation&&nextGeneration!==generation){owned=false;document.getElementById('control').textContent='Take control';}generation=nextGeneration;last=result.headers.get('X-Capture-Time')||last;state.textContent=(result.headers.get('X-Machine-ID')||'Linux guest')+' · LIVE · 1280×800 · last capture '+last;}
 }catch{if(!paused&&!document.hidden)unavailable('Disconnected — reconnecting automatically; VM lifecycle is unchanged');}
 finally{clearTimeout(deadline);setTimeout(tick,1000);}
}
tick();

const control=document.getElementById('control');
async function command(action,input){
 if(inputBusy)throw new Error('Input busy; wait for acknowledgement');
 inputBusy=true;const controller=new AbortController(),deadline=setTimeout(()=>controller.abort(),8000);
 try{const r=await fetch('/api/input',{method:'POST',headers:{'Authorization':'Bearer '+token,'Content-Type':'application/json'},body:JSON.stringify({action,generation,input}),signal:controller.signal});const v=await r.json();if(!r.ok)throw new Error(v.code||'Input unavailable');owned=!!v.owned;control.textContent=owned?'Give control to agent':'Take control';if(action==='input')etag='';}
 catch(e){owned=false;control.textContent='Take control';state.textContent=String(e.message)+'; observe before retrying';throw e;}
 finally{clearTimeout(deadline);inputBusy=false;}
}
fetch('/api/config',{headers:{'Authorization':'Bearer '+token},cache:'no-store'}).then(r=>r.json()).then(v=>{interactive=!!v.interactive;if(interactive){control.hidden=false;document.getElementById('typing').hidden=false;image.alt='Interactive Linux guest desktop';document.getElementById('help').textContent='Take control to click, scroll, type or send shortcuts. Give control back for the agent. Input affects Linux only. Closing the viewer keeps Linux running; ownership expires within 15 seconds.';}}).catch(()=>{});
control.addEventListener('click',()=>command(owned?'release':'acquire').catch(()=>{}));
setInterval(()=>{if(owned&&!inputBusy&&!document.hidden)command('renew').catch(()=>{});},5000);
function point(e){const r=image.getBoundingClientRect(),scale=Math.min(r.width/1280,r.height/800),width=1280*scale,height=800*scale;const x=(e.clientX-r.left-(r.width-width)/2)/scale,y=(e.clientY-r.top-(r.height-height)/2)/scale;if(x<0||y<0||x>=1280||y>=800)return null;return {x:Math.floor(x),y:Math.floor(y)};}
image.addEventListener('click',e=>{if(!owned||inputBusy)return;const p=point(e);if(p)command('input',{kind:'click',frameId,...p,button:'left',clicks:1}).catch(()=>{});});
image.addEventListener('contextmenu',e=>{if(!owned)return;e.preventDefault();const p=point(e);if(p&&!inputBusy)command('input',{kind:'click',frameId,...p,button:'right',clicks:1}).catch(()=>{});});
image.addEventListener('wheel',e=>{if(!owned)return;e.preventDefault();const p=point(e);if(p&&!inputBusy)command('input',{kind:'scroll',frameId,...p,dx:0,dy:Math.sign(e.deltaY)}).catch(()=>{});},{passive:false});
document.getElementById('send').addEventListener('click',()=>{if(!owned||!frameId)return;const text=document.getElementById('text').value;command('input',{kind:'text',frameId,text,pasteMode:document.getElementById('terminalPaste').checked?'terminal':'standard'}).then(()=>{document.getElementById('text').value='';}).catch(()=>{});});
document.getElementById('press').addEventListener('click',()=>{if(owned&&frameId)command('input',{kind:'key',frameId,key:document.getElementById('key').value}).catch(()=>{});});
