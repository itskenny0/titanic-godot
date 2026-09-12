import {build} from 'esbuild';
import {readFileSync} from 'node:fs';
import path from 'node:path';
import {nativePixelsPlugin} from './native-pixels-plugin.mjs';
const root=process.cwd();
await build({entryPoints:['engine/player.ts'],bundle:true,format:'iife',target:'es2022',outfile:'godot/engine.js',
 banner:{js:readFileSync('engine/platform.js','utf8')},
 plugins:[nativePixelsPlugin(),{name:'engine-and-restored-saves',setup(b){
 b.onResolve({filter:/savegame$/},a=>{
  if(a.path==='@dreamfactory/engine/df/savegame'||(a.path==='../df/savegame'&&a.importer.endsWith('/runtime/saveload.ts')))
   return {path:path.join(root,'engine/restoration/save-format.ts')};
 });
 b.onResolve({filter:/^@dreamfactory\/engine\//},a=>({path:path.join(root,'vendor/dreamrefactory/engine/src',a.path.replace('@dreamfactory/engine/',''))+'.ts'}));
 }}]});
