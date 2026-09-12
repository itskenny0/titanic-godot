// Temporary migration oracle. Owned room imagery is represented by local hashes.
import {readFileSync,writeFileSync} from 'node:fs';import {createHash} from 'node:crypto';
import {readSetFile} from '../vendor/dreamrefactory/engine/src/df/set';
import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
import {SetViewer} from '../vendor/dreamrefactory/engine/src/web/viewer';
const paths=JSON.parse(readFileSync(process.argv[2],'utf8')).filter((x:any)=>x.path.toLowerCase().endsWith('.set')),output=[];
const hashes=new WeakMap<object,string>();const hash=(b:Uint8Array)=>{let h=hashes.get(b);if(!h){h=createHash('sha256').update(b).digest('hex');hashes.set(b,h)}return h};
const frame=(f:any)=>f?{width:f.width,height:f.height,pixels:hash(f.pixels),z:f.z?hash(f.z):'',camera:f.cam??null}:null;
for(const {path}of paths){const set=readSetFile(new Uint8Array(readFileSync(path)));if(!set.scenes.length||set.scenes.some(s=>!s.views.length))continue;const modes=[];
 for(const mode of ['original','sharp','soft','transition']){
  const s=new GameSession(()=>undefined,new NullAudioSink());s.pictureMode=mode as any;let v:SetViewer;
  const dir:any={screen:{},get busy(){return v?.roomAnimating??false},get movingCamera(){return v?.roomAnimating??false}};
  v=new SetViewer(set,s,'','',dir);(v.scripts as any).closeScene=async()=>{};(v.scripts as any).openScene=async()=>{};
  const state=()=>({scene:v.sceneIdx,view:v.viewIdx,animating:v.roomAnimating,frame:frame(v.roomFrame()),camera:v.roomCamera()});
  const stands=[],actions=[];
  for(let si=0;si<set.scenes.length;si++)for(let vi=0;vi<set.scenes[si].views.length;vi++){
   const scene=set.scenes[si],view=scene.views[vi];v.jumpTo(scene.sceneName,view.viewName);
   stands.push({...state(),roads:v.availableRoads().map(r=>({name:r.road.transitionName,register:r.register,arrive:r.arriveViewID})),listener:s.listener(),hits:view.objects.map(o=>{const x=(o.startRegionX+o.endRegionX)/2,y=(o.startRegionY+o.endRegionY)/2;return {x,y,hit:v.hitTest(x,y)?.obj.identifier??''}})});
   if(mode!=='original'&&si!==0)continue;
   for(const command of ['left','right','walk'])for(const pace of (si===0&&vi===0?[0,25,50,100]:[50])){
    v.jumpTo(scene.sceneName,view.viewName);await s.settle();
    if(command==='walk')v.walk(pace);else v.turn(command==='left'?1:0,pace);
    const steps=[{now:0,...state()}];let now=100,i=0;
    while(v.roomAnimating){if(i>10000)throw Error('animation did not settle');v.advanceRoom(now);steps.push({now,...state()});now+=[17,83,250,25,50][i++%5]}
    await s.settle();actions.push({scene:si,view:vi,command,pace,steps});
   }
  }
  modes.push({mode,stands,actions});
 }
 output.push({path,modes});
}
writeFileSync(process.argv[3],JSON.stringify(output));console.log(`Viewer reference: ${output.length} sets, ${output.reduce((n,s)=>n+s.modes.reduce((m,v)=>m+v.stands.length,0),0)} standpoints, ${output.reduce((n,s)=>n+s.modes.reduce((m,v)=>m+v.actions.length,0),0)} movements`);
