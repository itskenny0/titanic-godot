import {readFileSync} from 'node:fs';

// Retain the pinned upstream source and its fallback for runtimes without the helper.
export function nativePixelsPlugin() {
 return {name:'native-pixels',setup(build){
  build.onLoad({filter:/[/\\]df[/\\]image\.ts$/},args=>{
   const source=readFileSync(args.path,'utf8');
   const anchor='  const n = width * height;\n  const dst = out ?? new Uint8ClampedArray(n * 4);';
   if(source.split(anchor).length!==2)throw Error('Pinned pixel conversion function changed');
   const frame='  fb.ensure(width, height);';
   if(source.split(frame).length!==2)throw Error('Pinned frame decoder changed');
   return {loader:'ts',contents:source.replace(anchor,anchor+'\n  if (n > 0 && typeof globalThis.__indexedRGBA === "function") return globalThis.__indexedRGBA(indexed, paletteRGBA, dst, n);').replace(frame,frame+'\n  if (typeof globalThis.__decodeFrame === "function") { const zOffset = globalThis.__decodeFrame(data, fb.pixels, fb.zPixels, !le); return {width, height, hasZ: zOffset >= 0, zOffset}; }')};
  });
  build.onLoad({filter:/[/\\]df[/\\]audio\.ts$/},args=>{
   const source=readFileSync(args.path,'utf8');
   const anchor='  const { codec, sampleRate, byteSize, dataStart } = header;';
   if(source.split(anchor).length!==2)throw Error('Pinned audio decoder changed');
   return {loader:'ts',contents:source.replace(anchor,anchor+'\n  if (typeof globalThis.__decodeAudio === "function") { const samples = new Float32Array(codec === 1 ? byteSize : byteSize >> 1); globalThis.__decodeAudio(data, samples, !little(order)); return {sampleRate, samples}; }')};
  });
 }};
}
