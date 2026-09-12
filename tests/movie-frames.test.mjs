import test from 'node:test';
import assert from 'node:assert/strict';
import {build} from 'esbuild';
import {mkdir} from 'node:fs/promises';
await mkdir('.build/tests',{recursive:true});
await build({entryPoints:['engine/movie-frames.ts'],outfile:'.build/tests/movie-frames.mjs',bundle:true,platform:'node',format:'esm',alias:{'@dreamfactory/engine':'./vendor/dreamrefactory/engine/src'}});
const {MovieFrames}=await import('../.build/tests/movie-frames.mjs');

function segment(count=200,width=64,height=32){
 const containers=[],frames=[],expected=[];let pixels=new Uint8Array(width*height);
 for(let i=0;i<count;i++){
  const bytes=[height&255,height>>8,width&255,width>>8];
  // Independent first frame, then change a single row of the delta buffer.
  for(let y=0;y<height;y++)if(i===0||y===i%height){
   bytes.push(20,6,width-32,i%256);pixels.fill(i%256,y*width,(y+1)*width);
  }else bytes.push(40);
  frames.push({locationFrame:containers.length});containers.push({data:new Uint8Array(bytes)});expected.push(pixels.slice());
 }
 return {seg:{frames,file:{containers,order:'le'},dfV1:false},expected};
}

test('movie decoding is lazy, bounded, and survives reverse jumps and eviction',()=>{
 const {seg,expected}=segment(),frames=new MovieFrames(seg,8192,16384);
 assert.equal(frames.decodedFrames,0);
 assert.deepEqual(frames.get(0).pixels,expected[0]);assert.equal(frames.decodedFrames,1);
 const retained=frames.get(0);
 for(const i of [...expected.keys(),199,100,101,32,0,1,199,198,197,42,43]){
  assert.deepEqual(frames.get(i).pixels,expected[i],`frame ${i}`);
  assert.ok(frames.retainedBytes<=8192+16384+64*32*2);
 }
 assert.deepEqual(retained.pixels,expected[0],'later decoding mutated a presented image');
 assert.equal(frames.get(-1),undefined);assert.equal(frames.get(200),undefined);
 const n=frames.decodedFrames;frames.get(43);assert.equal(frames.decodedFrames,n,'stationary picture decoded again');
});
