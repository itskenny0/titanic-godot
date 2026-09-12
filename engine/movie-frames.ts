import {FrameBuffer,decodeFrame} from '@dreamfactory/engine/df/image';
import {compositeFrameV1} from '@dreamfactory/engine/df/mov-v1';
import type {MovSegment} from '@dreamfactory/engine/df/mov';

export interface MovieImage {pixels:Uint8Array;width:number;height:number;}
interface Checkpoint {pixels:Uint8Array;z:Uint8Array;width:number;height:number;shown:Uint8Array|null;}

// Decoding the supplied credits segment used about 158 MiB before it could start.
// Keep a few recent images and sparse restart points instead. The raw delta
// buffer is independent of presented images, so later frames cannot alter one
// still in use by the renderer.
export class MovieFrames {
 readonly length:number;
 private fb=new FrameBuffer();
 private position=-1;
 private shown:Uint8Array|null=null;
 private images=new Map<number,MovieImage>();
 private checkpoints=new Map<number,Checkpoint>();
 private imageBytes=0;
 private checkpointBytes=0;
 decodedFrames=0;
 constructor(private seg:MovSegment,private imageBudget=2*1024*1024,private checkpointBudget=4*1024*1024) {
  this.length=seg.frames.length;
 }
 get retainedBytes(){return this.imageBytes+this.checkpointBytes+this.fb.pixels.byteLength+this.fb.zPixels.byteLength+(this.shown?.byteLength||0);}
 get(index:number):MovieImage|undefined {
  if(!Number.isInteger(index)||index<0||index>=this.length)return undefined;
  const cached=this.images.get(index);
  if(cached){this.images.delete(index);this.images.set(index,cached);return cached;}
  if(index<=this.position)this.restore(index);
  let result:MovieImage|undefined;
  while(this.position<index) {
   const next=this.position+1,meta=this.seg.frames[next];
   const d=decodeFrame(this.seg.file.containers[meta.locationFrame].data,this.fb,this.seg.file.order);
   this.decodedFrames++;
   // Intermediate images are not needed unless v1 transparency composites them.
   if(next===index||this.seg.dfV1) {
    const pixels=this.fb.pixels.slice(0,d.width*d.height);
    if(this.seg.dfV1){compositeFrameV1(pixels,this.shown);this.shown=pixels;}
    if(next===index)result={pixels,width:d.width,height:d.height};
   }
   this.position=next;
   if(next%32===0)this.checkpoint();
  }
  if(result){this.images.set(index,result);this.imageBytes+=result.pixels.byteLength;this.trimImages();}
  return result;
 }
 private trimImages(){
  while(this.imageBytes>this.imageBudget&&this.images.size>1){const key=this.images.keys().next().value!;this.imageBytes-=this.images.get(key)!.pixels.byteLength;this.images.delete(key);}
 }
 private checkpoint(){
  if(this.checkpoints.has(this.position))return;
  const c={pixels:this.fb.pixels.slice(),z:this.fb.zPixels.slice(),width:this.fb.width,height:this.fb.height,shown:this.shown?.slice()??null};
  const bytes=c.pixels.byteLength+c.z.byteLength+(c.shown?.byteLength||0);
  if(bytes>this.checkpointBudget)return;
  this.checkpoints.set(this.position,c);this.checkpointBytes+=bytes;
  while(this.checkpointBytes>this.checkpointBudget){
   const key=this.checkpoints.keys().next().value!,old=this.checkpoints.get(key)!;
   this.checkpointBytes-=old.pixels.byteLength+old.z.byteLength+(old.shown?.byteLength||0);this.checkpoints.delete(key);
  }
 }
 private restore(index:number){
  let nearest=-1;
  for(const at of this.checkpoints.keys())if(at<index&&at>nearest)nearest=at;
  this.fb=new FrameBuffer();this.shown=null;this.position=nearest;
  if(nearest>=0){
   const c=this.checkpoints.get(nearest)!;
   this.fb.pixels=c.pixels.slice();this.fb.zPixels=c.z.slice();this.fb.width=c.width;this.fb.height=c.height;this.shown=c.shown?.slice()??null;
   this.checkpoints.delete(nearest);this.checkpoints.set(nearest,c);
  }
 }
}
