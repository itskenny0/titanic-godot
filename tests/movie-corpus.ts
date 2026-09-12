import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {MovieFrames} from '../engine/movie-frames';
import {readMovFile} from '../vendor/dreamrefactory/engine/src/df/mov';
const corpus=JSON.parse(readFileSync(process.argv[2],'utf8'));
let checked=0,peak=0,largest=0,largestFile='';
for(const entry of corpus){
 if(!entry.path.toLowerCase().endsWith('.mov'))continue;
 const movie=readMovFile(new Uint8Array(readFileSync(entry.path)));
 for(let i=0;i<movie.segments.length;i++){
  const seq=new MovieFrames(movie.segments[i]),refs=entry.rings[i];
  if(seq.length!==refs.length)throw Error('frame table differs: '+entry.path);
  const eager=refs.reduce((sum:any,f:any)=>sum+f.width*f.height,0);
  if(eager>largest){largest=eager;largestFile=entry.path;}
  // Sequential playback, then jumps across old checkpoints and backwards.
  const accesses=[...refs.keys()];
  if(refs.length)accesses.push(0,Math.floor(refs.length/2),refs.length-1,Math.max(0,refs.length-2),0);
  for(const index of accesses){
   const frame=seq.get(index)!;
   const hash=createHash('sha256').update(frame.pixels).digest('hex');
   if(hash!==refs[index].pixels)throw Error(`Movie mismatch ${entry.path} segment ${i} frame ${index}`);
   peak=Math.max(peak,seq.retainedBytes);checked++;
  }
 }
}
console.log(JSON.stringify({checked,peakDecodedBytes:peak,largestEagerSegmentBytes:largest,largestFile}));
