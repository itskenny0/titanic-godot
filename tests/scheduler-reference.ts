// Authored scenarios for migration comparisons, with no game files or saves.
import {writeFileSync} from 'node:fs';
import {Scheduler} from '../vendor/dreamrefactory/engine/src/runtime/scheduler';
import {ActorRuntime} from '../vendor/dreamrefactory/engine/src/runtime/actors';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
const actors=new ActorRuntime();actors.addCast('test',{members:['alice','bob','claire'].map(name=>({name,poses:[{name:'stand',play:[0,1],steps:[]},{name:'walk',play:[0,1,2],steps:[]}]}))} as any);
for(const a of actors.actors.values()){a.poseName='walk';a.worldX=-3;a.worldY=2;a.worldZ=-1;a.speed=7;a.turn=16;a.deg=250;}
const audio=new NullAudioSink(),events:any[]=[],pending:Promise<any>[]=[],logs:string[]=[];let busy=false,active=0;
const session:any={actorRuntime:actors,audio,audioLib:{sound:()=>({sampleRate:100,samples:new Float32Array(20)}),bankNames:['test']},ambientRng:()=>0.25,currentSetName:'room',currentFlat:'main',flatScripts:new Map(),castScripts:new Map([...actors.actors.keys()].map(n=>[n,{script:{codes:new Map([['endwalk',true]])}}])),listener:()=>({x:0,y:0,deg:0}),onLog:(s:string)=>logs.push(s),clock:{advance(){}},get scriptBusy(){return busy||active>0},hasGlobal:()=>true,runGlobal:async()=>{events.push(['global','calctime'])},sendEvent:async(cmd:string,target:string,handler:string,_args:any,caller:string)=>{events.push([cmd,target,handler,caller,!!session.navGestureActive])},track:(p:Promise<any>)=>{active++;pending.push(p.finally(()=>active--));return p},trackIdle:(p:Promise<any>)=>{pending.push(p);return p},onNavigate:'oldNav',onSceneJump:'oldScene',onViewJump:'oldView',navDriver:'nav',sceneJumpDriver:'scene',viewJumpDriver:'view',navGestureActive:false,navFromScript:false};
const scheduler=new Scheduler(session);
scheduler.startWalk('alice',-20,35,-3,'finish');scheduler.startWalkPath('bob',[{x:-3,y:2,z:-1,fromPrev:0},{x:12,y:-7,z:3,fromPrev:20},{x:24,y:10,z:8,fromPrev:30}],'routeEnd');scheduler.startTurn('claire',128);
scheduler.makeLoop('scene','scene2',' Animate ( ) ',3);scheduler.makeLoop('prop','counter','once()',1);scheduler.makeLoop('flat','missing','flatTick',2);
scheduler.makeCricket('rumble',3,4,10,2,3);scheduler.makeCricket('single',100,100,10,1,-1);scheduler.soundLoop('rumble',true);
const output=[];
for(let step=0;step<24;step++){
 busy=step>=2&&step<=5;
 if(step===4){scheduler.playSound('rumble',true);scheduler.playSound('rumble',true)}
 if(step===5)scheduler.soundLoop('rumble',false);
 if(step===7)scheduler.pauseWalk('bob',true);
 if(step===8)session.currentSetName='other';
 if(step===10){session.currentSetName='room';scheduler.pauseWalk('bob',false)}
 if(step===12)scheduler.haltSounds();
 scheduler.tickTime((step+1)*50);scheduler.serviceFrameLoops();
 while(pending.length)await Promise.all(pending.splice(0));
 output.push({step,actors:[...actors.actors.values()].map(a=>({name:a.name,x:a.worldX,y:a.worldY,z:a.worldZ,deg:a.deg,pose:a.poseName,step:a.step,star:a.starName})),walks:[...scheduler.walks.keys()],loops:scheduler.loops.map(l=>({...l})),crickets:scheduler.crickets.map(({handle,...c})=>({...c,done:handle?.done??true})),events:[...events],calls:audio.calls.map(c=>({...c})),navigation:[session.onNavigate,session.onSceneJump,session.onViewJump,session.navGestureActive,session.navFromScript]});
}
writeFileSync(process.argv[2],'[\n'+output.map(row=>JSON.stringify(row)).join(',\n')+'\n]\n');console.log('Scheduler reference: '+output.length+' steps');
