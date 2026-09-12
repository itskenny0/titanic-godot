// Temporary migration oracle. Save contents and restored state stay local.
import {readFileSync,readdirSync,writeFileSync} from 'node:fs';
import {join,basename} from 'node:path';
import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
import {createRestoredSnapshot} from '../engine/restoration/neutral-save';
const root=process.argv[2],index=new Map<string,string>();
for(const disc of ['cd1','cd2'])for(const e of readdirSync(join(root,disc),{recursive:true,withFileTypes:true})){if(!e.isFile())continue;index.set(`${disc.slice(2)}/${e.name.toLowerCase()}`,join(e.parentPath,e.name))}
const saves=JSON.parse(readFileSync(process.argv[3],'utf8')).saves,output=[];
for(const {path} of saves){
 let disc=1;const files=(name:string)=>{const n=basename(name).toLowerCase(),p=index.get(`${disc}/${n}`)??index.get(`${3-disc}/${n}`);return p?new Uint8Array(readFileSync(p)):undefined};
 const s=new GameSession(files,new NullAudioSink()),unknown:string[]=[],jumps:any[]=[];s.onLog=()=>{};s.interp.onUnknown=n=>unknown.push(n);s.seedRandom(123);s.onDiscChange=n=>disc=n;s.onSetChange=async(...args)=>{jumps.push(args)};
 await s.ensureBooted();s.interp.globals.set('unsaved_future',123);s.interp.globals.set('__keep',27);s.cursorDepth=-5;
 if(!await s.loadGame(new Uint8Array(readFileSync(path))))throw Error(`Could not load ${path}`);
 const snap=createRestoredSnapshot(s),off=new DataView(snap.buffer).getUint32(4,true)+17; // marker is 17 bytes
 const metadata=JSON.parse(new TextDecoder().decode(snap.subarray(off,snap.length-25)));
 output.push({path,metadata,disc,jumps,stage:s.stageName,flat:s.currentFlat,set:s.currentSetName,cursor:s.cursorDepth,internal:s.interp.globals.get('__keep'),unknown,props:[...s.propRuntime.props].map(([name,p])=>({name,index:p.frameIdx,order:p.frameOrder,locked:p.frameLocked,animating:p.animating}))});
}
writeFileSync(process.argv[4],JSON.stringify(output));console.log(`Save restoration reference: ${output.length} sessions`);
