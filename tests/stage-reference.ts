// Temporary migration oracle. Image hashes and stage metadata stay local.
import {readFileSync,writeFileSync} from 'node:fs';
import {extname} from 'node:path';
import {createHash} from 'node:crypto';
import {StageController} from '../vendor/dreamrefactory/engine/src/runtime/stage';
const hash=(b:Uint8Array)=>createHash('sha256').update(b).digest('hex');
const out=[];
for(const {path}of JSON.parse(readFileSync(process.argv[2],'utf8'))){if(extname(path).toLowerCase()!=='.stg')continue;const bytes=new Uint8Array(readFileSync(path));
 const session:any={stageName:'none',currentFlat:'none',flatNames:[],flatScripts:new Map(),fade:{blanked:false,level:0,queue:[],snapshot:null},onClut(){},ensureFile:async()=>{},files:()=>bytes,onLog(){},instanceFrom:()=>null,refreshFallbacks(){},fireHandler:async()=>{},sendEvent:async()=>0,clearTextOverlay(){},plugins:{reset(){}}};
 const stage=new StageController(session);if(!await stage.openStageFile('test.stg'))throw Error(path);const frames=[];
 for(const name of [...session.flatNames,...session.flatNames.slice().reverse()]){await stage.gotoFlat(name);const f=stage.flatImage();frames.push({name,index:stage.flatToIndex(name),width:f?.width,height:f?.height,pixels:f?hash(f.pixels):null,palette:f?hash(f.palette):null,regions:stage.currentFlatRegions()});}
 out.push({path,refName:stage.stageRefName(),frames});
}
writeFileSync(process.argv[3],JSON.stringify(out));console.log('Stage reference: '+out.length+' stages, '+out.reduce((n,s)=>n+s.frames.length,0)+' flat visits');
