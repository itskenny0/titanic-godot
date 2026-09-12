// Temporary migration oracle. Character art stays local as pixel hashes.
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
import {PuppetView} from '../vendor/dreamrefactory/engine/src/web/puppet-view';
import {DrawSignature} from '../vendor/dreamrefactory/engine/src/runtime/signature';
import {setScreenGamma} from '../vendor/dreamrefactory/engine/src/web/screen-gamma';
const output=[];
const pixels=new Uint8Array(512*264).map((_,i)=>i%256),palette=new Uint8ClampedArray(1024).map((_,i)=>i%4===3?255:(i*17)&255);
for(const {path} of JSON.parse(readFileSync(process.argv[2],'utf8'))){
 if(!path.toLowerCase().endsWith('.pup'))continue;
 const bytes=new Uint8Array(readFileSync(path)),s=new GameSession(()=>bytes,new NullAudioSink());
 if(!await s.puppetCtrl.openPuppetFile('test.pup'))throw Error(path);
 const p=s.puppet!,v=new PuppetView(s),cases=[];
 p.bevels=Array.from({length:6},(_,i)=>({text:`Answer ${i+1}`,id:i}));
 const poses=[{ident:'default',stance:p.stanceIdx,pose:p.pose},...Array.from(p.pup.dialogue.values()).map(line=>{s.puppetCtrl.puppetBase(line.ident);return{ident:line.ident,stance:p.stanceIdx,pose:p.pose}})];
 for(let i=0;i<poses.length;i++)for(const variant of (i===0?[0,1,2,3,4,5,6]:[0,1])){
  const pose=poses[i];p.stanceIdx=pose.stance;p.pose=pose.pose;p.anim=null;
  setScreenGamma(variant===5?1.2:.65);
  p.subtitle=variant===0?'':variant===6?'日本語の字幕、長い文章でも画面内に表示されるように改行する。'.repeat(3):'A long subtitle that must wrap across two lines while the character speaks to the player.';
  s.puppetParams.set(7,variant===2?0:1);p.chosen=variant===3?2:null;p.press=variant===4?{index:1,until:100}:null;
  const backdrop=variant===1||variant===5?null:{pixels,width:512,height:264,palette};
  const rgba=new Uint8ClampedArray(512*384*4).fill(37);v.composite(rgba,backdrop);
  const ctx:any={font:'12px Arial',fillStyle:'#fff',strokeStyle:'#fff',lineWidth:1,commands:[],stack:[],
   save(){this.stack.push([this.font,this.fillStyle,this.strokeStyle,this.lineWidth])},restore(){[this.font,this.fillStyle,this.strokeStyle,this.lineWidth]=this.stack.pop()},
   measureText(text:string){return {width:text.length*7}},
   fillText(text:string,x:number,y:number){this.commands.push({op:'text',text,x,y,w:0,h:0,font:this.font,color:this.fillStyle})},
   fillRect(x:number,y:number,w:number,h:number){this.commands.push({op:'rect',x,y,w,h,color:this.fillStyle})},
   strokeRect(x:number,y:number,w:number,h:number){this.commands.push({op:'stroke',x,y,w,h,color:this.strokeStyle,line:this.lineWidth})}};
  v.drawOverlay(ctx);const sig=new DrawSignature();sig.reset();v.drawSignature(sig);
  cases.push({ident:pose.ident,variant,pixels:createHash('sha256').update(rgba).digest('hex'),commands:ctx.commands,lo:sig.lo,hi:sig.hi});
 }
 output.push({path,cases});
}
writeFileSync(process.argv[3],JSON.stringify(output));console.log(`Puppet view reference: ${output.length} characters, ${output.reduce((n,e)=>n+e.cases.length,0)} composites`);
