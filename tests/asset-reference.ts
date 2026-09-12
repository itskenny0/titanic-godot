// Temporary migration oracle. Outputs stay local because they describe owned data.
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {extname} from 'node:path';
import {readSetFile,readStarPath} from '../vendor/dreamrefactory/engine/src/df/set';
import {readStgFile,readStgRegions} from '../vendor/dreamrefactory/engine/src/df/stg';
import {readShpFile,decodeShpFrame} from '../vendor/dreamrefactory/engine/src/df/shp';
import {readCstFile} from '../vendor/dreamrefactory/engine/src/df/cst';
import {readPupFile,readAnimLogic} from '../vendor/dreamrefactory/engine/src/df/pup';
import {readMovFile} from '../vendor/dreamrefactory/engine/src/df/mov';
import {readAudioBank} from '../vendor/dreamrefactory/engine/src/df/banks';
import {readContainerFile} from '../vendor/dreamrefactory/engine/src/df/container';
const readers:any={'.set':readSetFile,'.stg':readStgFile,'.shp':readShpFile,'.cst':readCstFile,'.pup':readPupFile,'.mov':(b:Uint8Array)=>readMovFile(b).segments,'.trk':(b:Uint8Array)=>readAudioBank(readContainerFile(b)),'.sfx':(b:Uint8Array)=>readAudioBank(readContainerFile(b)),'.11k':(b:Uint8Array)=>readAudioBank(readContainerFile(b))};
function normalize(v:any):any {
 if(v instanceof Uint8Array)return Buffer.from(v).toString('base64');
 if(v instanceof Map)return normalize(Object.fromEntries(v));
 if(Array.isArray(v))return v.map(normalize);
 if(v&&typeof v==='object')return Object.fromEntries(Object.entries(v).filter(([k])=>k!=='file'&&k!=='segments').map(([k,x])=>[k,normalize(x)]));
 return v;
}
const input=JSON.parse(readFileSync(process.argv[2],'utf8')),out=[];
const hash=(b:Uint8Array)=>createHash('sha256').update(b).digest('hex');
for(const {path}of input){
 const kind=extname(path).toLowerCase(),reader=readers[kind];if(!reader)continue;
 const bytes=new Uint8Array(readFileSync(path));
 try {
  const value=reader(bytes),entry:any={path,kind,expected:normalize(value)};
  if(kind==='.stg')entry.regions=value.flats.map((f:any)=>readStgRegions(value.file.containers[f.locationClickLogic]?.data??new Uint8Array(),value.version));
  if(kind==='.set')entry.paths=value.starPaths.map((p:any)=>({loc:p.container,points:readStarPath(value.file.containers,p.container,value.version)}));
  if(kind==='.pup')entry.animations=[...new Set([...value.dialogue.values()].map((d:any)=>d.animLogicLocation))].map((loc:any)=>({loc,frames:readAnimLogic(value,loc)}));
  if(kind==='.shp'||kind==='.cst'){
   const locations=kind==='.shp'?value.groups.flatMap((g:any)=>g.states.flatMap((s:any)=>s.frames)):value.members.flatMap((m:any)=>m.poses.flatMap((p:any)=>p.steps.flatMap((s:any)=>s.map((f:any)=>f.location))));
   entry.sprites=[...new Set(locations)].map((loc:any)=>{const f=decodeShpFrame(value.file.containers[loc].data);return {loc,width:f.width,height:f.height,posXraw:f.posXraw,posYraw:f.posYraw,indexed:hash(f.indexed),opaque:hash(f.opaque)}});
  }
  out.push(entry);
 }catch(e){throw Error(path+': '+e);}
}
writeFileSync(process.argv[3],JSON.stringify(out));console.log('Asset reference: '+out.length+' files');
