/* Browser reference only. No backend calls, credentials, storage, or external assets. */
(() => {
  'use strict';
  const F = globalThis.QUOTARY_FIXTURES;
  const $ = id => document.getElementById(id);
  const clone = x => JSON.parse(JSON.stringify(x));
  const clamp = (n,a,b) => Math.max(a,Math.min(b,n));
  const short = (s,n) => [...String(s)].length > n ? [...String(s)].slice(0,Math.max(0,n-1)).join('')+'…' : String(s);
  const pad = (s,n) => short(s,n).padEnd(n,' ');
  const esc = s => s.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;');
  const captions = {
    overview:'The subscription desk. Remaining capacity leads; all 24 sources stay within reach.',
    attention:'Source failures stay local. Stale readings retain their age; missing data never becomes zero.',
    detail:'One Google source, separate model groups. No combined Google quota.',
    history:'Muse history. Known tokens, a visible coverage gap, and no invented dollar estimate.',
    picker:'The source library. 106 provider IDs become 107 selectable sources with Google variants.',
    empty:'First launch. A direct path to tracking many subscriptions or exploring the demo.',
    backend:'Backend unavailable. Setup guidance leaves the rest of the interface usable.',
    help:'Keyboard navigation stays visible. The browser frame is a review tool, not part of the TUI.'
  };
  let state, grid, regions=[], timer;
  const initial = () => ({w:120,h:40,theme:'dark',view:'overview',scenario:'overview',selected:'claude:configured',filter:'all',query:'',editing:false,layout:'auto',page:0,detailOffset:0,helpOffset:0,pickerIndex:0,pickerQuery:'',pickerEditing:false,sort:0,notice:'',refreshing:false,previous:'overview',sources:clone(F.sources),tracked:new Set(F.sources.map(s=>s.id))});
  class Grid {
    constructor(w,h) { this.w=w;this.h=h;this.cells=Array.from({length:h},()=>Array.from({length:w},()=>[' ','text','canvas',false])); }
    text(x,y,text,fg='text',bg=null,bold=false,max=this.w-x) {
      if(y<0||y>=this.h)return;
      [...String(text)].slice(0,Math.max(0,max)).forEach((c,i)=>{const xx=x+i;if(xx>=0&&xx<this.w)this.cells[y][xx]=[c,fg,c==='█'?fg:bg||this.cells[y][xx][2],bold];});
    }
    right(x,y,text,fg='text',bg=null,bold=false) { this.text(x-[...text].length,y,text,fg,bg,bold); }
    fill(x,y,w,h,bg) {for(let yy=y;yy<y+h;yy++)this.text(x,yy,' '.repeat(w),'text',bg);}
    line(x,y,w,fg='border') {this.text(x,y,'─'.repeat(Math.max(0,w)),fg);}
    box(x,y,w,h,title='',selected=false) {
      this.fill(x,y,w,h,'surface');const c=selected?'accent':'border';
      this.text(x,y,'╭'+'─'.repeat(w-2)+'╮',c);
      for(let i=1;i<h-1;i++){this.text(x,y+i,'│',c);this.text(x+w-1,y+i,'│',c);}
      this.text(x,y+h-1,'╰'+'─'.repeat(w-2)+'╯',c);
      if(title)this.text(x+2,y,short(' '+title+' ',w-4),selected?'accent':'text','surface',true);
    }
    render() {
      return this.cells.map(row=>{
        let html='',run='',prev='';
        const flush=()=>{if(run){const [fg,bg,b]=prev.split('|');html+=`<span style="color:var(--${fg});background:var(--${bg});${b==='1'?'font-weight:700;':''}">${esc(run)}</span>`;}};
        for(const [c,fg,bg,bold] of row){const key=`${fg}|${bg}|${bold?1:0}`;if(key!==prev){flush();run='';prev=key;}run+=c;}flush();return html;
      }).join('\n');
    }
  }
  function region(x,y,w,h,action) {regions.push({x,y,w,h,action});}
  function color(s) {if(s.state==='stale')return 'stale';if(['signin','unavailable','exhausted'].includes(s.state))return 'error';if(s.state==='missing')return 'low';return s.windows.some(w=>w.left!==null&&w.left<=20)?'low':'good';}
  function status(s) {if(s.state!=='current')return {stale:'Stale',signin:'Sign-in',unavailable:'Unavailable',missing:'Partial',exhausted:'Exhausted'}[s.state];return s.windows.some(w=>w.left===0)?'Exhausted':s.windows.some(w=>w.left!==null&&w.left<=20)?'Low':'Current';}
  function attention(s) {return status(s)!=='Current';}
  function list() {
    let xs=state.sources.filter(s=>state.tracked.has(s.id));
    if(state.filter==='attention')xs=xs.filter(attention);
    if(state.filter==='pinned')xs=xs.filter(s=>s.pinned);
    xs=xs.filter(s=>(s.name+' '+s.plan).toLowerCase().includes(state.query.toLowerCase()));
    return xs.sort((a,b)=> state.sort===1?a.name.localeCompare(b.name):state.sort===2?Number(attention(b))-Number(attention(a))||a.name.localeCompare(b.name):state.sort===3?(a.windows[0]?.resetMinutes??Infinity)-(b.windows[0]?.resetMinutes??Infinity)||a.name.localeCompare(b.name):Number(b.pinned)-Number(a.pinned)||(a.pinned?a.pinOrder-b.pinOrder:a.name.localeCompare(b.name)));
  }
  function selected() {return state.sources.find(s=>s.id===state.selected)||state.sources[0];}
  function meter(x,y,length,left,fg=null) {
    if(left===null||left===undefined){grid.text(x,y,'Not reported','muted');return;}
    const full=Math.round(length*clamp(left,0,100)/100);
    grid.text(x,y,'█'.repeat(full),fg|| (left===0?'error':left<=20?'low':'good'));
    grid.text(x+full,y,'░'.repeat(length-full),'border');
  }
  function header(label='') {
    grid.text(2,1,'QUOTARY','accent',null,true);
    if(state.w>=100)grid.text(13,1,'Your AI subscriptions, at a glance','muted');
    grid.right(state.w-2,1,state.w<80?'DEMO · FICTIONAL':'DEMO · fictional data','muted');
    const tracked=state.sources.filter(s=>state.tracked.has(s.id));
    const current=tracked.filter(s=>['current','exhausted'].includes(s.state)).length;
    const low=tracked.filter(s=>['Low','Exhausted'].includes(status(s))).length;
    const problem=tracked.filter(s=>['signin','unavailable','stale','missing'].includes(s.state)).length;
    let summary=label||`${tracked.length} sources · ${current} current${problem?' · '+(low+problem)+' need attention':low?' · '+low+' low':''}`;
    if(state.w<80)summary=label||`${tracked.length} sources · ${current} current · ${low+problem} attention`;
    grid.text(2,3,short(summary,state.w-4),'muted');
    if(state.w>=100&&!label)grid.right(state.w-2,3,state.refreshing?'Refreshing · simulated':state.notice||'Updated just now',state.refreshing?'accent':'muted');
    grid.line(2,4,state.w-4);
  }
  function footer(text,position='') {
    grid.line(2,state.h-3,state.w-4);
    grid.text(2,state.h-2,short(text,state.w-4-(position.length?position.length+2:0)),'muted');
    if(position)grid.right(state.w-2,state.h-2,position,'accent');
  }
  function card(s,x,y,w,h) {
    const active=s.id===state.selected;
    grid.box(x,y,w,h,(active?'> ':'')+s.name,active);
    grid.text(x+2,y+1,short(s.plan+(s.pinned?' · pinned':''),w-4),'muted');
    if(s.state==='signin'||s.state==='unavailable'){
      grid.text(x+2,y+3,s.state==='signin'?'Sign-in needed':'Source unavailable','error',null,true);
      grid.text(x+2,y+4,'No current reading','muted');
      grid.text(x+2,y+6,s.state==='signin'?'Enter for setup guidance':'Enter to inspect and retry','muted');
    } else {
      s.windows.slice(0,2).forEach((a,i)=>{
        const yy=y+2+i*(h>=11?3:2), usedColor=s.state==='stale'?'stale':null;
        grid.text(x+2,yy,pad(a.label,6),'muted');
        if(a.left===null)grid.text(x+9,yy,'Not reported','muted');
        else {meter(x+9,yy,10,a.left,usedColor);grid.right(x+w-2,yy,`${a.left}% left`,usedColor||(a.left===0?'error':a.left<=20?'low':'text'),null,true);}
        grid.text(x+9,yy+1,short(a.reset?'Resets in '+a.reset:'Reset not reported',w-11),'muted');
      });
      if(s.windows.length===1)grid.text(x+2,y+5,s.windows[0].label==='Daily'?'Daily allowance':'Plan allowance','muted');
    }
    const extra=s.windows.length>2?`+${s.windows.length-2} more`:'';
    const st=status(s),bottom=st==='Current'?`Updated ${s.age} ago`:`${st} · ${s.age} ago`;
    grid.text(x+2,y+h-2,short(state.refreshing?'Refreshing · previous sample':bottom,w-4-extra.length-(extra?1:0)),st==='Current'?'muted':color(s));
    if(extra)grid.right(x+w-2,y+h-2,extra,'muted');
    region(x,y,w,h,()=>{if(state.selected===s.id)openDetail(s.id);else state.selected=s.id;});
  }
  function overview() {
    header();
    grid.text(2,6,'Overview','text',null,true);
    const label={all:'All tracked',attention:'Attention',pinned:'Pinned'}[state.filter];
    grid.text(state.w<80?14:16,6,`[ ${label} ]`,'accent');
    if(state.w>=88)grid.right(state.w-2,6,'/ Search   f Filter   p Sources','muted');
    region(16,6,20,1,cycleFilter);
    if(state.query||state.editing)grid.text(2,7,short('/ '+state.query+(state.editing?'▏':''),state.w-4),'accent');
    const xs=list();
    if(!xs.length){grid.text(4,11,state.query?'No sources match your search.':'No sources in this view.','text',null,true);grid.text(4,13,short(state.query?`Query: ${state.query}`:'Press p to choose subscriptions.',state.w-8),'muted');grid.text(4,15,'Esc clears search · f changes the filter','muted');footer('p Sources  / Search  ? Help  q Quit');return;}
    if(!xs.some(s=>s.id===state.selected))state.selected=xs[0].id;
    const compact=state.layout==='compact'||state.layout==='auto'&&(state.w<88||state.h<32);
    if(compact){compactList(xs);return;}
    const cols=state.w>=160?4:state.w>=120?3:state.w>=88?2:1;
    const w=Math.floor((state.w-4-(cols-1)*2)/cols);
    const h=state.h>=48?11:9;
    const rows=Math.max(1,Math.floor((state.h-10)/(h+1)));
    const count=cols*rows,index=xs.findIndex(s=>s.id===state.selected);
    state.page=Math.floor(index/count);const start=state.page*count;
    xs.slice(start,start+count).forEach((s,i)=>card(s,2+(i%cols)*(w+2),8+Math.floor(i/cols)*(h+1),w,h));
    const end=Math.min(start+count,xs.length);
    footer(state.w>=120?'↑↓←→ Navigate   Enter Details   Space Pin   r Refresh   l Layout   ? Help   q Quit':'↑↓←→ Move  Enter Details  p Sources  l Layout  ? Help  q Quit',`${start+1}–${end} / ${xs.length}`);
  }
  function compactList(xs) {
    const narrow=state.w<80, max=Math.max(1,state.h-16);
    const idx=xs.findIndex(s=>s.id===state.selected),start=Math.floor(idx/max)*max;
    const nameW=narrow?24:27,leftX=narrow?29:46,resetX=narrow?36:55,statusX=state.w-13;
    grid.text(4,8,'Source','muted');
    if(!narrow)grid.text(32,8,'Allowance','muted');
    grid.text(leftX,8,'Left','muted');grid.text(resetX,8,narrow?'Reset / state':'Resets','muted');
    if(!narrow)grid.text(statusX,8,'State','muted');
    let y=10,visible=0;
    for(const s of xs.slice(start,start+max)){
      if(y>=state.h-5)break;
      const active=s.id===state.selected,a=s.windows[0];
      if(active)grid.fill(2,y,state.w-4,1,'surface');
      grid.text(2,y,active?'>':' ',active?'accent':'text');
      grid.text(4,y,short(s.name,nameW),active?'accent':'text',null,active);
      const absent=['signin','unavailable'].includes(s.state);
      if(!narrow)grid.text(32,y,absent?'—':short(a.label,11),'muted');
      grid.right(leftX+4,y,absent||a.left===null?'—':`${a.left}%`,color(s),null,true);
      grid.text(resetX,y,short(absent?(narrow?status(s):'—'):narrow&&s.state==='stale'?`${s.age} old`:a.reset||'Unknown',narrow?state.w-resetX-2:11),absent?'error':s.state==='stale'?'stale':'muted');
      if(!narrow)grid.text(statusX,y,status(s),color(s));
      region(2,y,state.w-4,1,()=>{if(active)openDetail(s.id);else state.selected=s.id;});
      y++;visible++;
      if(active&&s.windows[1]&&!absent){const b=s.windows[1];grid.text(narrow?5:32,y,narrow?b.label:short(b.label,11),'muted');grid.right(leftX+4,y,b.left===null?'—':b.left+'%',s.state==='stale'?'stale':'muted');grid.text(resetX,y,b.reset||'Unknown','muted');y++;}
    }
    grid.text(2,state.h-4,short(`${selected().name} · ${selected().plan} · ${status(selected())}`,state.w-4),'muted');
    footer(narrow?'↑↓ Move  Enter Open  p Sources  ? Help  q Quit':'↑↓ Move  Enter Details  / Search  p Sources  ? Help  q Quit',`${start+1}–${start+visible}/${xs.length}`);
  }
  function dateFrom(minutes) {if(minutes===null||minutes===undefined)return 'Not reported';return new Date(Date.parse(F.clock)+minutes*60000).toISOString().replace('T',' ').slice(0,16)+' UTC';}
  function openDetail(id,history=false) {state.selected=id;state.view=history?'history':'detail';state.detailOffset=0;}
  function detail() {
    const s=selected(),wide=state.w>=100;
    header(`${s.name} · ${s.plan}`);
    grid.text(2,6,'‹ Overview','muted');region(2,6,12,1,()=>state.view='overview');
    grid.text(17,6,'[ Allowances ]','accent',null,true);grid.text(34,6,s.history?'History':'History unavailable','muted');
    if(s.history)region(34,6,10,1,()=>state.view='history');
    if(['signin','unavailable'].includes(s.state)){
      grid.box(2,9,state.w-4,Math.min(15,state.h-14),s.name,true);
      grid.text(5,11,s.state==='signin'?'Sign-in needed':'The source could not be reached','error',null,true);
      const lines=['No current allowance reading is available.','','Sign in using this provider’s supported CodexBar setup.','Return here and press r to retry.','','Your other subscriptions can keep refreshing.'];
      lines.forEach((t,i)=>grid.text(5,13+i,short(t,state.w-10),'muted'));
    } else {
      const bodyW=wide?state.w-42:state.w-4,x=2;
      const available=state.h-13,perPage=Math.max(1,Math.floor(available/5));
      const start=clamp(state.detailOffset,0,Math.max(0,s.windows.length-perPage));
      s.windows.slice(start,start+perPage).forEach((a,i)=>{
        const y=9+i*5;
        grid.text(x+1,y,short((a.group?a.group+' · ':'')+a.label,bodyW-20),'text',null,true);
        grid.right(x+bodyW-2,y,a.left===null?'Not reported':`${a.left}% left`,s.state==='stale'?'stale':a.left===null?'muted':a.left===0?'error':a.left<=20?'low':'good',null,true);
        if(a.left!==null)meter(x+1,y+1,bodyW-3,a.left,s.state==='stale'?'stale':null);
        grid.text(x+1,y+2,short(a.reset?`Resets in ${a.reset} · ${dateFrom(a.resetMinutes)}`:'Reset time not reported',bodyW-3),'muted');
        grid.line(x+1,y+4,bodyW-3);
      });
      if(wide){
        const sx=state.w-37;
        grid.box(sx,9,35,13,'Reading');
        grid.text(sx+2,11,'STATUS','muted');grid.text(sx+2,12,status(s),color(s),null,true);
        const age=Number.parseInt(s.age,10)*(s.age.endsWith('m')?60:1);
        const sampled=new Date(Date.parse(F.clock)-age*1000).toISOString().slice(11,19);
        grid.text(sx+2,14,'LAST SUCCESS · 16 OCT','muted');grid.text(sx+2,15,`${sampled} UTC · ${s.age} ago`,'text');
        grid.text(sx+2,17,'ACCOUNT','muted');grid.text(sx+2,18,'Hidden · privacy mode','text');
        grid.text(sx+2,20,'r Refresh this source','accent');
        grid.text(sx+2,24,'ABOUT THIS READING','muted');
        const notes=s.provider==='antigravity'?['Model groups have separate','allowances. Gemini app and','Gemini CLI are other sources.']:['Only this source is shown.','Allowances keep their own','units and reset periods.'];
        notes.forEach((t,i)=>grid.text(sx+2,26+i,t,'muted'));
      }
      const bottom=state.h-5;
      grid.text(3,bottom,short(`Source: ${s.id} · ${s.state==='stale'?'Previous sample retained':'Fictional reference reading'}`,state.w-6),'muted');
      if(s.windows.length>perPage)grid.text(3,bottom-1,'↑↓ Scroll allowances','accent');
    }
    footer(state.w<80?`Esc Back  ${s.history?'Tab History  ':''}↑↓ Scroll  r Refresh  ? Help`:`Esc Overview   ${s.history?'Tab Switch tab   ':''}↑↓ Scroll   r Refresh source   ? Help   q Quit`);
  }
  function history() {
    const s=selected(),narrow=state.w<100;
    header(`${s.name} · ${s.plan}`);
    grid.text(2,6,'‹ Overview','muted');grid.text(17,6,'Allowances','muted');grid.text(31,6,'[ History ]','accent',null,true);
    region(17,6,12,1,()=>state.view='detail');region(2,6,12,1,()=>state.view='overview');
    if(!s.history){grid.text(3,11,'History is unavailable for this source.','muted');footer('Esc Overview   Tab Allowances   ? Help   q Quit');return;}
    grid.text(3,8,'Last 7 days · 10–16 Oct 2026 · UTC','text',null,true);
    grid.text(3,10,short('Partial coverage · this machine only · 12 Oct unavailable',state.w-6),'low');
    if(state.h<38){
      grid.text(3,12,'4.74M known tokens · 6 of 7 days','accent',null,true);
      grid.text(3,14,'Daily tokens: 0.42M 0.61M ? 0.98M','muted');
      grid.text(3,15,'              0.72M 1.18M 0.83M','muted');
      if(state.h>=24){grid.text(3,18,'Model A  2.50M · Model B  1.89M · Model C  350k','muted');}
      grid.text(3,state.h-5,'? means unavailable; it does not mean zero.','muted');
      footer('Esc Back  Tab Allowances  r Refresh  ? Help');return;
    }
    grid.text(3,12,'4.74M','accent',null,true);grid.text(12,12,'known tokens','muted');
    grid.text(3,13,'6 of 7 days with recorded data','muted');
    const chartW=narrow?state.w-7:Math.floor(state.w*.58),base=24,height=7;
    const step=Math.floor((chartW-7)/7),left=11;
    grid.text(3,16,'TOKENS / DAY','muted');grid.text(3,18,'1.2M','muted');grid.text(3,24,'0','muted');
    F.history.days.forEach((d,i)=>{
      const x=left+i*step;
      if(d.tokens===null){grid.text(x+2,base-2,'?','low');grid.text(x,base,'┄'.repeat(step-2),'low');}
      else {const rows=Math.round(d.tokens/1200000*height);for(let j=0;j<rows;j++)grid.text(x,base-j,'█'.repeat(Math.max(1,step-3)),i===5?'accent':'good');}
      grid.text(x,base+2,d.day.slice(-2),'muted');
    });
    grid.text(3,28,short('Daily totals: 420k · 610k · ? · 980k · 720k · 1.18M · 830k',state.w-6),'muted');
    grid.text(3,30,'? Unavailable day · known total excludes this gap','low');
    if(!narrow){
      const x=chartW+6,w=state.w-x-3;
      grid.box(x,12,w,15,'Token breakdown');
      grid.text(x+2,14,'Model','muted');grid.right(x+w-2,14,'Tokens','muted');
      F.history.models.forEach((m,i)=>{grid.text(x+2,16+i*2,m.name,'text');grid.right(x+w-2,16+i*2,((m.input+m.output)/1000000).toFixed(2)+'M','text');});
      grid.line(x+2,22,w-4);grid.text(x+2,24,'Known subtotal','muted');grid.right(x+w-2,24,'4.74M','accent',null,true);
    }
    if(state.h>=38){grid.text(3,33,s.provider==='muse'?'Muse supplies token history; dollar totals are not available.':'This reference supplies tokens only; price estimates are unavailable.','muted');grid.text(3,34,'Fictional model labels · local activity is not subscription consumption.','muted');}
    footer('Esc Overview   Tab Allowances   r Refresh history   ? Help   q Quit');
  }
  function library(query=state.pickerQuery) {
    const fixtures=new Map(F.sources.map(s=>[s.provider,s]));
    let entries=F.catalog.flatMap(c=>{
      if(c.provider==='gemini')return [{id:'gemini:web',provider:'gemini',name:'Google · Gemini app',kind:'Subscription'},{id:'gemini:api',provider:'gemini',name:'Google · Gemini CLI',kind:'Subscription'}];
      const f=fixtures.get(c.provider),api=['openai','azureopenai','vertexai','claudeplatform','bedrock','deepseek'].includes(c.provider);
      return [{id:c.provider+':configured',provider:c.provider,name:f?.name||c.displayName,kind:f?'Subscription':api?'API usage':['openrouter','replicate','fal'].includes(c.provider)?'Credits':'Unclassified'}];
    });
    const priorities=['claude:configured','codex:configured','muse:configured','gemini:web','gemini:api','antigravity:configured'];
    entries.sort((a,b)=>{const aa=priorities.indexOf(a.id),bb=priorities.indexOf(b.id);return (aa<0?99:aa)-(bb<0?99:bb)||a.name.localeCompare(b.name);});
    return entries.filter(c=>(c.name+' '+c.kind).toLowerCase().includes(query.toLowerCase()));
  }
  function picker() {
    header('Source library · 106 providers · 107 selectable sources');
    grid.text(2,6,'Track the subscriptions you use.','text',null,true);
    if(state.w>=100)grid.right(state.w-2,6,`${state.tracked.size} tracked`,'accent');
    grid.text(2,8,'[ Track all subscriptions ]','accent',null,true);region(2,8,27,1,trackAll);
    if(state.w>=88)grid.text(33,8,'Tracking does not sign you in.','muted');
    grid.text(2,10,short('/ '+(state.pickerQuery||'Search the full catalog')+(state.pickerEditing?'▏':''),state.w-4),state.pickerEditing?'accent':'muted');
    const xs=library(),max=Math.max(1,state.h-20);state.pickerIndex=clamp(state.pickerIndex,0,Math.max(0,xs.length-1));
    const start=Math.floor(state.pickerIndex/max)*max;
    const wide=state.w>=100,kindX=wide?48:state.w-20;
    grid.text(7,12,'Source','muted');grid.text(kindX,12,'Kind','muted');
    if(wide)grid.text(69,12,'Last reading','muted');
    xs.slice(start,start+max).forEach((s,i)=>{
      const y=14+i,sel=start+i===state.pickerIndex;
      if(sel)grid.fill(2,y,state.w-4,1,'surface');
      grid.text(2,y,sel?'>':' ','accent');grid.text(4,y,state.tracked.has(s.id)?'[x]':'[ ]',state.tracked.has(s.id)?'accent':'muted');
      grid.text(9,y,short(s.name,kindX-11),sel?'accent':'text',null,sel);grid.text(kindX,y,s.kind,'muted');
      if(wide){const f=state.sources.find(f=>f.id===s.id);grid.text(69,y,f?status(f):s.provider==='azureopenai'?'Manual setup · active probe':'No reading',f?color(f):'muted');}
      region(2,y,state.w-4,1,()=>{state.pickerIndex=start+i;toggleTracked();});
    });
    if(!xs.length)grid.text(3,15,'No catalog sources match this search.','muted');
    grid.text(2,state.h-5,short('API, credits and unclassified sources stay separate from subscriptions.',state.w-4),'muted');
    grid.text(2,state.h-4,short('Catalog availability does not confirm an active subscription.',state.w-4),'muted');
    footer(state.w<80?'↑↓ Move  Space Track  / Search  Esc Back':'↑↓ Move   Space Track   a Track subscriptions   / Search   Esc Back   ? Help',`${Math.min(start+1,xs.length)}–${Math.min(start+max,xs.length)}/${xs.length}`);
  }
  function trackAll() {library('').filter(s=>s.kind==='Subscription').forEach(s=>state.tracked.add(s.id));state.notice='Subscriptions tracked';}
  function toggleTracked() {const s=library()[state.pickerIndex];if(!s)return;if(state.tracked.has(s.id))state.tracked.delete(s.id);else {state.tracked.add(s.id);if(!state.sources.some(f=>f.id===s.id))state.sources.push({...s,plan:'Plan not reported',windows:[],state:'unavailable',age:'—',pinned:false,pinOrder:999,history:false});}}
  function empty(backend=false) {
    header(backend?'Setup · no backend connection':'Welcome · no sources tracked');
    if(state.h<28){
      grid.text(3,7,backend?'CodexBar could not be found.':'Your subscriptions, together.','accent',null,true);
      grid.text(3,9,backend?'Set the executable in Settings.':'Choose sources to build your desk.','muted');
      grid.text(3,11,backend?'[ , Open settings ]':'[ p Choose sources ]','accent');
      grid.text(3,13,'[ d Explore the demo ]','accent');
      footer('p Sources   d Demo   , Settings   ? Help   q Quit');return;
    }
    const w=Math.min(state.w-4,84),x=Math.floor((state.w-w)/2),y=8;
    if(state.h>=28){grid.box(x,y,w,21,backend?'Connect CodexBar':'Your subscriptions, together.',true);}
    grid.text(x+3,y+2,backend?'CodexBar could not be found.':'Make room for your whole toolkit.','text',null,true);
    const lines=backend?['Quotary needs CodexBar to read your subscriptions.','','Set the executable in Settings, or add codexbar to PATH.','Keep its resource bundle beside the executable.','','[ , Open settings ]     [ d Explore the demo ]']:['Choose sources from the library to build your desk.','Claude, Codex, Muse, Google, and many more.','','[ p Choose sources ]    [ d Explore the demo ]'];
    lines.forEach((t,i)=>grid.text(x+3,y+4+i,short(t,w-6),t.startsWith('[')?'accent':'muted'));
    if(state.h>=28){grid.text(x+3,y+14,'Track all subscriptions in one action.','text');grid.text(x+3,y+16,'Available sources and active logins are separate.','muted');grid.text(x+3,y+17,'Quotary never invents an allowance for a missing reading.','muted');}
    footer('p Sources   d Demo   , Settings   ? Help   q Quit');
  }
  function help() {
    header('Keyboard guide');
    grid.text(3,7,'A keyboard-first desk.','accent',null,true);
    const keys=[['↑↓←→ / j k','Move between sources'],['Enter / Esc','Open details / return'],['Tab / Shift+Tab','Switch details tabs'],['/','Search the current collection'],['f / s','Cycle filter / sort'],['Space','Pin source / track in library'],['p / l','Source library / layout'],['r / R','Refresh selected / all'],['PgUp / PgDn','Page through the collection'],[',','Open settings'],['? / q / Ctrl+C','Help / quit']];
    const step=state.h>=38?2:1,count=Math.max(1,Math.floor((state.h-14)/step));
    state.helpOffset=clamp(state.helpOffset,0,Math.max(0,keys.length-count));
    keys.slice(state.helpOffset,state.helpOffset+count).forEach(([key,action],i)=>{grid.text(3,10+i*step,pad(key,20),'accent');grid.text(25,10+i*step,short(action,state.w-28),'text');});
    if(state.h>=38)grid.text(3,state.h-6,'In search, letters type text. Esc returns to navigation.','muted');
    footer('Esc Return   ↑↓ Scroll   q Quit');
  }
  function settings() {
    header('Settings');grid.text(3,8,'Appearance','text',null,true);
    grid.text(3,10,`[ ${state.theme} ]    ← → Change theme`,'accent');
    grid.text(3,14,'Refresh every 5 minutes','text');grid.text(3,16,'Privacy mode: hide account identity','text');
    if(state.h>=28){grid.text(3,20,'Backend path: resolved from configuration / PATH','muted');grid.text(3,23,'Reference controls only; no preferences are written.','muted');}
    footer('←→ Appearance   Esc Return   ? Help   q Quit');
  }
  function cycleFilter() {const filters=['all','attention','pinned'];state.filter=filters[(filters.indexOf(state.filter)+1)%3];state.page=0;}
  function simulateRefresh(all=false) {
    if(state.refreshing)return;state.refreshing=true;state.notice='';render();
    clearTimeout(timer);timer=setTimeout(()=>{state.refreshing=false;state.notice=all?'Demo refresh complete':'Source refreshed · demo';render();},850);
  }
  function render() {
    grid=new Grid(state.w,state.h);regions=[];
    ({overview,detail,history,picker,empty:()=>empty(false),backend:()=>empty(true),help,settings,quit:()=>{header('Demo paused');grid.text(3,9,'Reference session ended. Press Enter to return.','muted');footer('Enter Restart');}}[state.view]||overview)();
    $('terminal').innerHTML=grid.render();$('terminal-frame').dataset.theme=state.theme;
    $('dimensions').textContent=`${state.w} columns × ${state.h} rows`;
    $('caption').textContent=captions[state.scenario]||captions.overview;
    $('theme').value=state.theme;$('size').value=state.w+'×'+state.h;
    fit();
  }
  function fit() {
    const frame=$('terminal-frame'),stage=$('stage');
    if(document.body.classList.contains('capture'))return;
    const scale=Math.min(1,stage.clientWidth/frame.offsetWidth);
    frame.style.transform=`scale(${scale})`;stage.style.height=frame.offsetHeight*scale+'px';
    $('scale-note').textContent=scale<.99?`Preview scaled to ${Math.round(scale*100)}% · exports are full size`:'One character per terminal cell';
  }
  function setScene(scene) {
    const keep={w:state.w,h:state.h,theme:state.theme};state={...initial(),...keep,scenario:scene};
    if(scene==='attention'){
      const transforms=globalThis.QUOTARY_SCENARIOS?.attention||{};
      for(const [provider,patch] of Object.entries(transforms)){
        const source=state.sources.find(s=>s.provider===provider);
        if(!source)continue;
        if(patch.windows!==undefined)source.windows=clone(patch.windows);
        if(patch.age!==undefined)source.age=patch.age;
        if(patch.state!==undefined)source.state=patch.state;
        if(patch.windowIndex!==undefined)source.windows=source.windows.map((a,i)=>i===patch.windowIndex?{...a,left:patch.windowLeft}:a);
      }
      state.filter='attention';state.selected='antigravity:configured';
    }else if(scene==='detail'){state.view='detail';state.selected='antigravity:configured';}
    else if(scene==='history'){state.view='history';state.selected='muse:configured';}
    else if(['picker','help','backend','empty'].includes(scene)){state.view=scene;if(scene==='empty')state.tracked.clear();}
    $('screen').value=scene;render();
  }
  function move(delta) {
    if(state.view==='help'){state.helpOffset+=Math.sign(delta);return;}
    if(state.view==='picker'){state.pickerIndex=clamp(state.pickerIndex+delta,0,library().length-1);return;}
    if(state.view==='detail'){state.detailOffset=clamp(state.detailOffset+Math.sign(delta),0,selected().windows.length-1);return;}
    const xs=list(),idx=xs.findIndex(s=>s.id===state.selected);if(xs.length)state.selected=xs[clamp(idx+delta,0,xs.length-1)].id;
  }
  $('terminal').addEventListener('keydown',e=>{
    const k=e.key;
    if((e.ctrlKey&&k.toLowerCase()==='c')){e.preventDefault();state.view='quit';render();return;}
    if(state.editing||state.pickerEditing){
      e.preventDefault();const key=state.view==='picker'?'pickerQuery':'query';
      if(k==='Escape'||k==='Enter'){state.editing=false;state.pickerEditing=false;}
      else if(k==='Backspace')state[key]=state[key].slice(0,-1);
      else if(k.length===1&&!e.ctrlKey&&!e.metaKey)state[key]+=k;
      state.pickerIndex=0;render();return;
    }
    e.preventDefault();
    const cols=state.layout==='compact'||state.w<88||state.h<32?1:state.w>=160?4:state.w>=120?3:2;
    const pageSize=state.layout==='compact'||state.layout==='auto'&&(state.w<88||state.h<32)?Math.max(1,state.h-16):cols*Math.max(1,Math.floor((state.h-10)/(state.h>=48?12:10)));
    if(state.view==='quit'){if(k==='Enter')setScene('overview');return;}
    if(k==='Escape'){
      if(state.view==='overview'){state.query='';state.filter='all';}
      else if(['help','settings'].includes(state.view))state.view=state.previous;
      else state.view='overview';
    }else if(k==='?'){if(state.view==='help')state.view=state.previous;else {state.previous=state.view;state.view='help';}}
    else if(k==='q')state.view='quit';
    else if(k===','){state.previous=state.view;state.view='settings';}
    else if(state.view==='settings'&&['ArrowLeft','ArrowRight'].includes(k)){const a=['dark','light','mono'];state.theme=a[(a.indexOf(state.theme)+(k==='ArrowRight'?1:2))%3];}
    else if(k==='p')state.view='picker';
    else if(k==='d'&&['empty','backend'].includes(state.view))setScene('overview');
    else if(k==='/'){if(state.view==='picker'){state.pickerEditing=true;state.pickerQuery='';}else if(state.view==='overview'){state.editing=true;state.query='';}}
    else if(k==='ArrowDown')move(state.view==='overview'?cols:1);
    else if(k==='ArrowUp')move(state.view==='overview'?-cols:-1);
    else if(k==='j')move(1);
    else if(k==='k')move(-1);
    else if(k==='ArrowLeft')move(-1);
    else if(k==='ArrowRight')move(1);
    else if(k==='PageDown')move(state.view==='picker'?state.h-20:pageSize);
    else if(k==='PageUp')move(state.view==='picker'?20-state.h:-pageSize);
    else if(k==='Enter'&&state.view==='overview')openDetail(state.selected);
    else if(k==='Enter'&&state.view==='picker')toggleTracked();
    else if(k==='Tab'&&['detail','history'].includes(state.view)&&selected().history)state.view=state.view==='detail'?'history':'detail';
    else if(k===' '){if(state.view==='picker')toggleTracked();else if(state.view==='overview')selected().pinned=!selected().pinned;}
    else if(k==='a'&&state.view==='picker')trackAll();
    else if(k==='f'&&state.view==='overview')cycleFilter();
    else if(k==='s'&&state.view==='overview')state.sort=(state.sort+1)%4;
    else if(k==='l'&&state.view==='overview')state.layout=state.layout==='compact'?'cards':'compact';
    else if(k==='r'||k==='R')simulateRefresh(k==='R');
    render();
  });
  $('terminal').addEventListener('click',e=>{
    const rect=$('terminal').getBoundingClientRect(),x=Math.floor((e.clientX-rect.left)/(rect.width/state.w)),y=Math.floor((e.clientY-rect.top)/(rect.height/state.h));
    [...regions].reverse().find(r=>x>=r.x&&x<r.x+r.w&&y>=r.y&&y<r.y+r.h)?.action();render();$('terminal').focus({preventScroll:true});
  });
  $('screen').addEventListener('change',e=>setScene(e.target.value));
  $('size').addEventListener('change',e=>{[state.w,state.h]=e.target.value.split('×').map(Number);state.layout='auto';render();});
  $('theme').addEventListener('change',e=>{state.theme=e.target.value;render();});
  $('layout').addEventListener('click',()=>{state.layout=state.layout==='compact'?'cards':'compact';render();});
  $('reset').addEventListener('click',()=>setScene($('screen').value));
  window.addEventListener('resize',fit);
  const params=new URLSearchParams(location.search);state=initial();
  if(params.has('size')){const dims=params.get('size').split('x').map(Number);if(dims.length===2&&dims[0]>=60&&dims[0]<=200&&dims[1]>=20&&dims[1]<=60)[state.w,state.h]=dims;}
  if(['dark','light','mono'].includes(params.get('theme')))state.theme=params.get('theme');
  if(params.has('capture'))document.body.classList.add('capture');
  setScene(Object.hasOwn(captions,params.get('scene'))?params.get('scene'):'overview');
  if(params.get('page')==='2'){state.selected='augment:configured';render();}
  // A deterministic review/export API. It never accesses the backend.
  window.quotaryReference={setScene,setSize:(w,h)=>{state.w=w;state.h=h;state.layout='auto';render();},setTheme:theme=>{state.theme=theme;render();},text:()=>$('terminal').textContent,state:()=>({...state,sources:clone(state.sources),tracked:[...state.tracked]}),render};
})();
