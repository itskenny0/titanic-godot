import {GameHost, type HostFiles} from '@dreamfactory/engine/web/host';
import {installRestoredSaving} from './restoration/neutral-save';
import {parseSave} from './restoration/save-format';
import type {AudioSink, AudioChannel, PlayOpts, PlayHandle} from '@dreamfactory/engine/runtime/audio';
import type {DecodedAudio} from '@dreamfactory/engine/df/audio';
import {ALL_CHANNELS,resetScreenGamma,stepScreenGamma} from '@dreamfactory/engine/web/screen-gamma';

declare const __native:(method:string,args:string,buffer?:ArrayBuffer)=>string|ArrayBuffer|null;
const call=(method:string,args:any={})=>JSON.parse(__native(method,JSON.stringify(args)) as string || 'null');
const read=(path:string):Uint8Array|null=>{const b=__native('read',JSON.stringify({path}));return b instanceof ArrayBuffer?new Uint8Array(b):null;};
const write=(path:string,bytes:Uint8Array)=>{const r=JSON.parse(__native('write',JSON.stringify({path}),bytes.slice().buffer) as string);if(r?.error)throw Error(r.error);};
const events:any[]=[];
const emit=(type:string,data:any={})=>events.push({type,...data});
const fail=(e:any)=>{emit('log',{text:String(e)+(e?.stack?'\n'+e.stack:'')});emit('error',{text:String(e?.message||e)});};
export class Files implements HostFiles {
 disc:1|2=1; cache=new Map<string,Uint8Array>();
 constructor(readonly index:Record<string,string>){}
 key(name:string){return `${this.disc}/${name.toLowerCase().replace(/^.*[\\/:]/,'')}`;}
 provide=(name:string)=>{const key=this.key(name);if(this.cache.has(key))return this.cache.get(key)!;const p=this.index[key]||this.index[`${this.disc===1?2:1}/${key.slice(2)}`];const bytes=p?read(p):null;if(bytes)this.cache.set(key,bytes);return bytes;};
 async load(name:string){return this.provide(name);}
 setDisc(d:1|2){this.disc=d;}
 activeDisc(){return this.disc;}
 textEncoding(){return 'macintosh' as const;}
 has(name:string){return this.cache.has(this.key(name));}
 evict(name:string){const key=this.key(name),n=this.cache.get(key)?.length||0;this.cache.delete(key);return n;}
}
let nextAudio=0;
const audioBuffers=new Map<number,ArrayBuffer>();
class Audio implements AudioSink {
 entries=new Map<number,{done:boolean,channel:AudioChannel,overlap:boolean}>();
 play(channel:AudioChannel,audio:DecodedAudio,opts:PlayOpts={}):PlayHandle {
  if(!opts.overlap)this.halt(channel);
  const id=++nextAudio,entry={done:false,channel,overlap:!!opts.overlap};this.entries.set(id,entry);
  const pan=Math.max(-1,Math.min(1,opts.pan||0));
  const pcm=new ArrayBuffer(audio.samples.length*4),dv=new DataView(pcm);
  const gain=Math.max(0,Math.min(1,opts.volume??1));
  const l=gain*Math.cos((pan+1)*Math.PI/4),r=gain*Math.sin((pan+1)*Math.PI/4);
  for(let i=0;i<audio.samples.length;i++){const s=Math.max(-1,Math.min(1,audio.samples[i]))*32767;dv.setInt16(i*4,Math.round(s*l),true);dv.setInt16(i*4+2,Math.round(s*r),true);}
  audioBuffers.set(id,pcm);emit('audio_play',{id,channel,rate:audio.sampleRate,loop:!!opts.loop,samples:audio.samples.length});
  return {get done(){return entry.done;},stop:()=>{entry.done=true;this.entries.delete(id);emit('audio_stop',{id});}};
 }
 halt(channel:AudioChannel){for(const [id,e]of this.entries)if(e.channel===channel&&!e.overlap){e.done=true;this.entries.delete(id);emit('audio_stop',{id});}}
 isDone(channel:AudioChannel){return ![...this.entries.values()].some(e=>e.channel===channel&&!e.overlap&&!e.done);}
 setChannelVolume(channel:AudioChannel,volume:number){emit('audio_volume',{channel,volume});}
 setSuspended(on:boolean){emit('audio_pause',{on});}
 finish(id:number){const e=this.entries.get(id);if(e)e.done=true;this.entries.delete(id);}
}
const audio=new Audio();
const context:any={canvas:{width:512,height:384},font:'12px Arial',fillStyle:'#fff',strokeStyle:'#fff',lineWidth:1,commands:[],stack:[],version:0,
 putImageData(){this.commands=[];this.version++;},
 save(){this.stack.push([this.font,this.fillStyle,this.strokeStyle,this.lineWidth]);},
 restore(){[this.font,this.fillStyle,this.strokeStyle,this.lineWidth]=this.stack.pop();},
 fillText(text:string,x:number,y:number){this.commands.push({op:'text',text,x,y,font:this.font,color:this.fillStyle});},
 measureText(text:string){return {width:call('measure',{text,font:this.font})};},
 fillRect(x:number,y:number,w:number,h:number){this.commands.push({op:'rect',x,y,w,h,color:this.fillStyle});},
 strokeRect(x:number,y:number,w:number,h:number){this.commands.push({op:'stroke',x,y,w,h,color:this.strokeStyle,line:this.lineWidth});}
};
let host:GameHost, ready=false, pauseOwned=false, paused=false, busy=false, renderVersion=0;
let profiling=false,profileTick=0,profileRender=0;
let frameWaiters:(()=>void)[]=[];
let nextDialog=0;
const dialogs=new Map<number,(v:any)=>void>();
async function dialog(kind:string,text:string,value=''):Promise<any>{const id=++nextDialog;emit('dialog',{id,kind,text,value});return new Promise(resolve=>dialogs.set(id,resolve));}
async function frozen<T>(fn:()=>Promise<T>){const owns=!host.session.frozen;if(owns)host.session.freezeTime();try{return await fn();}finally{if(owns)host.session.thawTime();}}
async function save(bytes:Uint8Array){
 const name=await dialog('save','Name this saved game.',`${host.session.currentSetFile||'Voyage'} ${new Date().toISOString().slice(0,19).replace(/[:T]/g,'-')}`);
 if(!name)return;parseSave(bytes);write('save:'+name,bytes);emit('saved',{name});
}
async function chooseSave():Promise<Uint8Array|null>{
 const path=await dialog('load','Choose a saved game.');if(!path)return null;
 const bytes=read(path);if(!bytes)throw Error('Unable to read this save.');parseSave(bytes);return bytes;
}
async function boot(config:any){
 const files=new Files(config.index);
 host=new GameHost(files,audio,{log:text=>emit('log',{text}),hud:text=>emit('status',{text}),showStage:()=>emit('stage')});
 if(config.testing)(globalThis as any).titanicTesting={host,read,parseSave};
 const s=host.session;s.pictureMode='sharp';s.hasRealFrames=true;
 s.nextFrame=()=>new Promise(resolve=>frameWaiters.push(resolve));
 installRestoredSaving(s);
 s.onNoteDialog=async text=>{await frozen(()=>dialog('note',text));};
 s.onQuestionDialog=async text=>!!(await frozen(()=>dialog('question',text)));
 s.onTextDialog=async(text,initial)=>(await frozen(()=>dialog('text',text,initial)))||'';
 s.onQuit=()=>emit('quit');
 s.onSaveGame=async bytes=>{try{await save(bytes);}catch(e){fail(e);}};
 s.onLoadGame=async()=>{try{return await chooseSave();}catch(e){fail(e);return null;}};
 host.director.onCursor=name=>emit('cursor',{name});
 await host.preload();ready=true;emit('ready');
 if(config.save){const bytes=read(config.save);if(!bytes)throw Error('Cannot read save');parseSave(bytes);await host.loadSavedGame(bytes);}
 else await s.track(host.coldBoot(),'coldBoot');
}
function tick(dt:number){
 if(!host)return;
 const started=profiling?Date.now():0;
 if(!paused){(globalThis as any).__time+=dt;const now=(globalThis as any).__time;
 const timers=(globalThis as any).__timers;(globalThis as any).__timers=[];
 for(const t of timers)if(t.at<=now)t.fn();else(globalThis as any).__timers.push(t);
 host.director.tick(now);const waiters=frameWaiters;frameWaiters=[];for(const r of waiters)r();
 const renderStarted=profiling?Date.now():0;
 host.director.render(context);
 if(profiling){profileTick+=renderStarted-started;profileRender+=Date.now()-renderStarted;}
 }
}
async function command(c:any){
 if(c.action==='reply'){const resolve=dialogs.get(c.id);dialogs.delete(c.id);resolve?.(c.value);return;}
 if(c.action==='audio_done'){audio.finish(c.id);return;}
 if(!host)return;const s=host.session,d=host.director;
 if(c.action==='pause'){
  if(c.on===paused)return;paused=c.on;
  if(paused){pauseOwned=!s.frozen;if(pauseOwned)s.freezeTime();s.pointerDown=false;d.release(-1,-1);}
  else {if(pauseOwned)s.thawTime();pauseOwned=false;}return;
 }
 if(!ready||paused)return;
 if(c.action==='pointer'){s.shiftDown=!!c.shift;s.setPointer(c.x,c.y);
  if(c.kind==='press'){s.pointerDown=true;await s.track(d.press(c.x,c.y),'pointer press');}
  else if(c.kind==='release'){s.pointerDown=false;d.release(c.x,c.y);}
  else if(!s.pointerDown)await d.hover(c.x,c.y);return;
 }
 if(c.action==='key'){
  const key=c.key;if(['uparrow','leftarrow','rightarrow'].includes(key)&&host.viewer&&(s.viewShowing||!s.stageCtrl.keydownTarget()))await s.track(host.viewer.pressNav(key),'navigation');
  else await s.track(d.keyDown(key,!!c.special),'key');return;
 }
 if(c.action==='gamma'){if(c.key===9)resetScreenGamma();else stepScreenGamma(c.key%2===0,c.key<3?ALL_CHANNELS:[c.key<5,c.key>=5&&c.key<7,c.key>=7]);return;}
 if(busy)return;busy=true;
 try{
 if(c.action==='save'){
  if(!s.currentSetFile||d.inputLocked||!s.viewShowing)throw Error('Finish the conversation or animation, then save while exploring.');
  await frozen(async()=>{const bytes=s.snapshotSave();if(!bytes)throw Error('Unable to save');await save(bytes);});
 }
 if(c.action==='load'||c.action==='import')await frozen(async()=>{
  const path=c.path||await dialog(c.action==='import'?'import':'load','Choose a saved game.');if(!path)return;
  const bytes=read(path);if(!bytes)throw Error('Cannot read saved game.');parseSave(bytes);
  // Restart the VM only after validation. This also abandons any old script continuations.
  if(c.action==='import')write('save:'+path.replace(/\\/g,'/').split('/').pop(),bytes);
  emit('restart',{save:path});
 });
 if(c.action==='new')await frozen(async()=>{if(await dialog('question','Return to the main menu? Unsaved progress will be lost.'))emit('restart');});
 }finally{busy=false;}
}
Object.assign(globalThis,{
 titanicProfile:(enabled:boolean)=>{profiling=enabled;},
 titanicTimings:()=>{const result=JSON.stringify({tick_ms:profileTick,render_ms:profileRender});profileTick=0;profileRender=0;return result;},
 titanicBoot:(c:any)=>void boot(c).catch(fail),
 titanicTick:tick,
 titanicCommand:(c:any)=>void command(c).catch(fail),
 titanicEvents:()=>JSON.stringify(events.splice(0)),
 titanicFrame:()=>{if(context.version===renderVersion)return null;renderVersion=context.version;return host.screen.frame.buffer;},
 titanicOverlay:()=>JSON.stringify(context.commands),
 titanicAudio:(id:number)=>{const b=audioBuffers.get(id);audioBuffers.delete(id);return b;},
 titanicState:()=>JSON.stringify(host?{ready,set:host.session.currentSetFile,scene:host.session.currentSceneName(),view:host.session.currentViewName(),disc:host.files.activeDisc?.(),movie:host.director.movieFile,choices:host.director.choices,regions:host.director.movieRegions,frame:host.session.frameCounter,paused,inputLocked:host.director.inputLocked,viewShowing:host.session.viewShowing,frozen:host.session.frozen}:{}),
});
