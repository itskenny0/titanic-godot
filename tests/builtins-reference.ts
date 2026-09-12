// Temporary migration oracle; this fixture uses only authored synthetic values.
import {writeFileSync} from 'node:fs';
import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
const s=new GameSession(()=>undefined,new NullAudioSink());s.onLog=()=>{};s.seedRandom(123);
s.listener=()=>({x:-123,y:456,deg:511});s.hitTestAt=(x,y)=>({name:`hit:${x},${y}`,type:'actor'});s.pointInSet=(x,y)=>x>=0&&y>=0;s.pointInStage=(x,y)=>x<0;
const states=[{identifier:'open',animated:true,frames:[1,2,3,4],degrees:[0,0,1,1],refScales:[]},{identifier:'dial',animated:false,frames:[1,2,3],degrees:[0,85,170],refScales:[]},{identifier:'plain',animated:true,frames:[1,2,3],degrees:[],refScales:[]}];
const shop=s.propRuntime.addShop('test.shp',{groups:[{name:'door',states}]} as any);shop.frame=()=>({width:4,height:3,posXraw:2,posYraw:1,indexed:new Uint8Array(12),opaque:new Uint8Array(12).fill(1)} as any);
const calls:any[]=[];const units=(v:string)=>Array.from({length:v.length},(_,i)=>v.charCodeAt(i));
const val=(v:any)=>typeof v==='string'?{text:units(v)}:{num:v??0};
const state=()=>[...s.propRuntime.props].map(([name,p])=>({name,state:p.stateName,visible:p.visible,hidden:p.hidden,x:p.anchorX,y:p.anchorY,world:p.worldSpace,wx:p.worldX,wy:p.worldY,wz:p.worldZ,scale:p.scale,deg:p.deg,degEvent:p.degEvent,variants:p.degVariants,locked:p.frameLocked,animating:p.animating,index:p.frameIdx,order:p.frameOrder,owner:p.owner,value:p.value}));
async function call(name:string,...args:any[]){const result=await s.interp.builtins.get(name)!(s.interp,args,{} as any,undefined as any);calls.push({name,args,result:val(result),props:state()})}
for(const text of ['', 'alpha,beta,,delta','Café','A😀B']){
 await call('stringlength',text);
 for(const sep of ['',','])for(const idx of [-1,0,1,2,3,4,5,1.5]){await call('findword',text,sep,idx);await call('putword',text,sep,idx,'Z')}
}
for(const name of ['random','sqrt','stringtonum','numtostring'])for(const n of [0,1,17,256,'12.5','14abc',-3])await call(name,n);
for(const n of [-129,0,64,127,128,255,256,511])for(const mag of [-32769,-100,1,100,32767,32768])for(const name of ['calcvectx','calcvecty'])await call(name,n,mag);
for(const name of ['cameraxyz','playerxyz'])for(const axis of [0,1,2,3,4])await call(name,axis);
await call('currentdeg');
for(const [x,y]of [[0,0],[-1,-2],[32768,-32769],[100,200]]){await call('makepoint',x,y);const p=((x&65535)<<16)|(y&65535);for(const n of ['pointx','pointy','hittest','pointinset','pointinstage'])await call(n,p);await call('result');await call('calcdeg',0,p);await call('calcdist',0,p)}
for(const name of ['stageparam','setparam']){await call(name,7);await call(name,7,'test');await call(name,7)}
for(const name of ['menuvisible','keyaborts']){await call(name);await call(name,1);await call(name);await call(name,0)}
await call('propvisible','door',1);await call('propview','door','open');await call('propdeg','door',1);await call('propview','door','plain');await call('propview','door','dial');await call('propdeg','door',170);await call('propxy','door',12.5,20);await call('pointinprop','door',(12<<16)|20);await call('propowner','door','inventory');await call('propvalue','door',37);await call('propinstance','door','copy');await call('prophide','all');await call('propvisible','copy',1);await call('prophide','all',0);await call('propset','copy','Room');await call('propxyz','copy',-20,30,40);await call('propdeg','copy',64);await call('propscale','copy','1.5');await call('indextoprop',2);await call('result');await call('propdelete','door');await call('countprops');
for(const name of ['soundvol','soundpan']){await call(name,'test');await call(name,'test',17);await call(name,'TEST')}
for(const name of ['currentvoice','sounddone','voicedone','currentsound','countsounds','counttracks'])await call(name);

const sceneSetup=calls.length;
s.currentSetName='room';s.currentSceneName=()=> 'Scene1';s.currentViewName=()=> 'Front';
s.actorRuntime.addCast('test.cst',{members:[{name:'alice',poses:[]}]} as any);
for(const name of ['currentset','currentscene','currentview','setvisible','stagevisible','currentstage','currentflat','countflats','countscenes','countshops','currenttheme','framerate','wavevolume','themevol','hidecursor','showcursor','shiftkey','optionkey','commandkey'])await call(name);
await call('framerate',7);await call('framerate');await call('wavevolume',6.5);await call('wavevolume');await call('themevol','test',80);await call('themevol');await call('camerahi','-4.6');await call('camerahi');await call('indextoflat',0);await call('setvisible',0);await call('setvisible');await call('setvisible',1);
for(const name of ['actorexists','actorvisible','actorstar','actorpose','actordist','actoris3d','actorvalue','actorowner'])await call(name,'alice');
await call('actorvisible','alice',1);await call('actorxyz','alice',-7,3,4);for(const axis of [0,1,2,3,4])await call('actorxyz','alice',axis);await call('actordist','alice');await call('actorxy','alice',17,23);await call('actorxy','alice',2);await call('actoris3d','alice');await call('actordeg','alice',-17);await call('actordeg','alice');await call('actorinstance','alice','bob');await call('countactors');await call('indextoactor',2);await call('actorhide','alice');await call('actordist','alice');await call('actorvalue','bob',42);await call('actorvalue','bob');await call('actorowner','bob','alice');await call('actorowner','bob');await call('walktoxyz','bob',10,20,30);await call('iswalk','bob');await call('walkdest','bob');await call('countwalks');await call('indextowalk',1);await call('pausewalk','bob');await call('stopwalk','bob');await call('walkdest','bob');await call('walktostar','alice','12,-23,4');await call('walkdest','alice');await call('turntodeg','bob',64);await call('iswalk','bob');await call('actordelete','bob');await call('countactors');
calls[sceneSetup].setupScene=true;
writeFileSync(process.argv[2],'[\n'+calls.map(c=>JSON.stringify(c)).join(',\n')+'\n]\n');console.log(`Builtin reference: ${calls.length} calls`);
