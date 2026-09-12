// Temporary migration oracle. Owned-data globals and object state stay local.
import {readFileSync,readdirSync,writeFileSync} from 'node:fs';
import {join,basename} from 'node:path';
import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
const root=process.argv[2],index=new Map<string,string>();
for(const disc of ['cd1','cd2'])for(const entry of readdirSync(join(root,disc),{recursive:true,withFileTypes:true})){if(!entry.isFile())continue;const key=entry.name.toLowerCase();if(!index.has(key))index.set(key,join(entry.parentPath,entry.name))}
const files=(name:string)=>{const p=index.get(basename(name).toLowerCase());return p?new Uint8Array(readFileSync(p)):undefined};
const session=new GameSession(files,new NullAudioSink()),unknown:string[]=[];session.onLog=()=>{};session.interp.onUnknown=n=>unknown.push(n);session.seedRandom(123);
await session.ensureBooted();
const state={globals:[...session.interp.globals],shops:[...session.propRuntime.shops.keys()],casts:[...session.actorRuntime.casts.keys()],stage:session.stageName,flat:session.currentFlat,tracks:session.audioLib.bankNames,props:[...session.propRuntime.props].map(([name,p])=>({name,owner:p.owner,value:p.value,view:p.stateName,visible:p.visible,deg:p.deg,scale:p.scale,x:p.anchorX,y:p.anchorY})),actors:[...session.actorRuntime.actors].map(([name,a])=>({name,owner:a.owner,value:a.value,pose:a.poseName,visible:a.visible,deg:a.deg,set:a.setName,star:a.starName})),unknown};
writeFileSync(process.argv[3],JSON.stringify(state));console.log(`Native session boot oracle: ${state.props.length} props, ${state.actors.length} actors, ${state.globals.length} globals; unknown commands: ${unknown.join(', ')||'none'}`);
