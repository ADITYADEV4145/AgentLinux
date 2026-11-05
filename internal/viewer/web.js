'use strict';
// The fragment capability never enters an HTTP URL/log or persistent browser storage.
let token=location.hash.slice(1);history.replaceState(null,'','/');
const image=document.getElementById('desktop'),state=document.getElementById('state'),empty=document.getElementById('empty'),button=document.getElementById('pause');
let paused=false,etag='',objectURL='',last='';
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
  if(result.status===200||result.status===304){last=result.headers.get('X-Capture-Time')||last;state.textContent=(result.headers.get('X-Machine-ID')||'Linux guest')+' · LIVE · 1280×800 · last capture '+last;}
 }catch{if(!paused&&!document.hidden)unavailable('Disconnected — reconnecting automatically; VM lifecycle is unchanged');}
 finally{clearTimeout(deadline);setTimeout(tick,1000);}
}
tick();
