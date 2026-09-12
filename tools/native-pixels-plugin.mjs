import {readFileSync} from 'node:fs';

// Retain the pinned upstream source and its fallback for runtimes without the helper.
export function nativePixelsPlugin() {
 return {name:'native-pixels',setup(build){
  build.onLoad({filter:/[/\\]df[/\\]image\.ts$/},args=>{
   const source=readFileSync(args.path,'utf8');
   const anchor='  const n = width * height;\n  const dst = out ?? new Uint8ClampedArray(n * 4);';
   if(source.split(anchor).length!==2)throw Error('Pinned pixel conversion function changed');
   return {loader:'ts',contents:source.replace(anchor,anchor+'\n  if (n > 0 && typeof globalThis.__indexedRGBA === "function") return globalThis.__indexedRGBA(indexed, paletteRGBA, dst, n);')};
  });
 }};
}
