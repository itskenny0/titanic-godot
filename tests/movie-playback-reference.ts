// Temporary migration oracle. Owned movie samples are represented by hashes.
import {readFileSync,writeFileSync} from 'node:fs';import {createHash} from 'node:crypto';
import {readMovFile} from '../vendor/dreamrefactory/engine/src/df/mov';
import {frameHoldMs,frameWaits,segmentOnScreenMs,segmentInterval,bedRuntimeMs} from '../vendor/dreamrefactory/engine/src/df/mov-pace';
import {segmentAudio,soundtrackFor} from '../vendor/dreamrefactory/engine/src/df/mov-sound';
import {MovieFrames} from '../engine/movie-frames';
const hash=(b:Uint8Array|Float32Array)=>createHash('sha256').update(new Uint8Array(b.buffer,b.byteOffset,b.byteLength)).digest('hex');const out=[];
for(const {path}of JSON.parse(readFileSync(process.argv[2],'utf8'))){if(!path.toLowerCase().endsWith('.mov'))continue;
 const m=readMovFile(new Uint8Array(readFileSync(path))),segments=[];
 for(let i=0;i<m.segments.length;i++){
  const s=m.segments[i],audio=segmentAudio(s),interval=segmentInterval(s,s.frames.length,audio?.audioSec??0,i),bedMs=bedRuntimeMs(m,i),bed=audio?soundtrackFor(s,audio,interval,s.frames.length,bedMs):null;
  const stream=new MovieFrames(s),visits=[];
  if(s.frames.length)for(const index of [...s.frames.keys(),0,s.frames.length-1,Math.floor(s.frames.length/2),Math.min(32,s.frames.length-1),0]){
   const f=stream.get(index)!;visits.push({index,width:f.width,height:f.height,pixels:hash(f.pixels),retained:stream.retainedBytes,decoded:stream.decodedFrames});
  }
  segments.push({interval,bedMs,onScreenMs:segmentOnScreenMs(s),holds:s.frames.map((_,j)=>frameHoldMs(s,j)),waits:s.frames.map((_,j)=>frameWaits(s,j)),
   audio:audio?{rate:audio.rate,seconds:audio.audioSec,unique:audio.unique.map(hash),resampled:audio.resampled.map(hash)}:null,
   bed:bed?{rate:bed.sampleRate,samples:bed.samples.length,pixels:hash(bed.samples),loop:bed.loop}:null,visits});
 }
 out.push({path,segments});
}
writeFileSync(process.argv[3],JSON.stringify(out));console.log(`Movie playback reference: ${out.length} files, ${out.reduce((n,m)=>n+m.segments.reduce((n,s)=>n+s.visits.length,0),0)} frame visits`);
