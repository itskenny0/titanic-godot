// Produces hashes from the pinned reference. Owned game bytes stay outside Git.
import {readFileSync, readdirSync, writeFileSync} from 'node:fs';
import {resolve, join, extname} from 'node:path';
import {createHash} from 'node:crypto';
import {readContainerFile, writeContainerFile} from '../vendor/dreamrefactory/engine/src/df/container';
import {readSetFile} from '../vendor/dreamrefactory/engine/src/df/set';
import {readMovFile} from '../vendor/dreamrefactory/engine/src/df/mov';
import {decodeFrame, FrameBuffer} from '../vendor/dreamrefactory/engine/src/df/image';
import {readAudioHeader, decodeAudioContainer} from '../vendor/dreamrefactory/engine/src/df/audio';

const hash=(bytes:Uint8Array)=>createHash('sha256').update(bytes).digest('hex');
function* walk(dir:string):Generator<string> {
 for(const e of readdirSync(dir,{withFileTypes:true})) {
  const p=join(dir,e.name);if(e.isDirectory())yield* walk(p);else if(e.isFile())yield p;
 }
}
const out=process.argv[2], roots=process.argv.slice(3), seen=new Set<string>(), results:any[]=[];
let frames=0,audio=0;
for(const root of roots)for(const path of walk(resolve(root))) {
 if(!['.set','.mov','.trk','.sfx','.11k','.pup','.shp','.cst','.stg'].includes(extname(path).toLowerCase()))continue;
 const bytes=new Uint8Array(readFileSync(path)),digest=hash(bytes);
 if(seen.has(digest))continue;seen.add(digest);
 const file=readContainerFile(bytes), entry:any={path,roundtrip:hash(writeContainerFile(file)),rings:[],audio:[]};
 let rings:number[][]=[];
 if(extname(path).toLowerCase()==='.set') {
  const set=readSetFile(bytes);
  rings=[...set.scenes.flatMap(s=>s.turns.map(t=>t.frames.map(f=>f.frameContainerLoc).filter(Boolean))),...set.transitions.flatMap(t=>t.frameRegisters.map(r=>r.frames.map(f=>f.frameContainerLoc).filter(Boolean)))];
 } else if(extname(path).toLowerCase()==='.mov') {
  const mov=readMovFile(bytes);
  rings=mov.segments.map(s=>s.frames.map(f=>f.locationFrame).filter(Boolean));
 }
 for(const ring of rings) {
  const fb=new FrameBuffer(), decoded=[];
  for(const loc of ring) {
   const f=decodeFrame(file.containers[loc].data,fb,file.order),n=f.width*f.height;
   decoded.push({loc,width:f.width,height:f.height,pixels:hash(fb.pixels.subarray(0,n)),z:f.hasZ?hash(fb.zPixels.subarray(0,n)):'',zOffset:f.zOffset});frames++;
  }
  entry.rings.push(decoded);
 }
 for(let loc=0;loc<file.containers.length;loc++) {
  const data=file.containers[loc].data,h=readAudioHeader(data,file.order);
  if(!h||h.sampleRate<4000||h.sampleRate>96000||h.dataStart<48||h.dataStart>=data.length||h.byteSize<=0||h.byteSize>128*1024*1024)continue;
  const a=decodeAudioContainer(data,file.order);
  entry.audio.push({loc,rate:a.sampleRate,pcm:hash(new Uint8Array(a.samples.buffer)),samples:a.samples.length});audio++;
 }
 results.push(entry);
}
writeFileSync(out,JSON.stringify(results));
console.log(`Reference: ${results.length} files, ${frames} frames, ${audio} audio chunks`);
