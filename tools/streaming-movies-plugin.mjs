import {readFileSync} from 'node:fs';
import path from 'node:path';

export function streamingMoviesPlugin(){
 return {name:'streaming-movies',setup(build){
  build.onResolve({filter:/^retanic-movie-frames$/},()=>({path:path.resolve('engine/movie-frames.ts')}));
  build.onLoad({filter:/[/\\]web[/\\]movie-player\.ts$/},args=>{
   let source=readFileSync(args.path,'utf8');
   const start=source.indexOf('    const fb = new FrameBuffer();\n    const frames: MovieImage[] = [];');
   const end=source.indexOf('    if (!frames.length) return false;',start);
   if(start<0||end<0)throw Error('Pinned movie decoder changed');
   source=source.slice(0,start)+'    const frames = new MovieFrames(seg);\n'+source.slice(end);
   for(const [before,after] of [
    ['    frames: MovieImage[];','    frames: MovieFrames;'],
    ['m.frames[Math.min(m.pos, m.frames.length - 1)]','m.frames.get(Math.min(m.pos, m.frames.length - 1))'],
    ['m.frames[m.pos]','m.frames.get(m.pos)'],
    ['this.active.frames[this.active.pos]','this.active.frames.get(this.active.pos)']
   ]){if(source.split(before).length!==2)throw Error('Pinned movie frame access changed: '+before);source=source.replace(before,after);}
   return {loader:'ts',contents:'import {MovieFrames} from "retanic-movie-frames";\n'+source};
  });
 }};
}
