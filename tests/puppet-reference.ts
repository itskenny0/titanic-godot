// Temporary migration oracle. Keep dialogue and animation metadata local.
import {readFileSync,writeFileSync} from 'node:fs';
import {extname} from 'node:path';
import {PuppetController} from '../vendor/dreamrefactory/engine/src/runtime/puppet';
import {Clock} from '../vendor/dreamrefactory/engine/src/runtime/clock';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
const out=[];
for(const {path}of JSON.parse(readFileSync(process.argv[2],'utf8'))){if(extname(path).toLowerCase()!=='.pup')continue;const bytes=new Uint8Array(readFileSync(path)),audio=new NullAudioSink(),clock=new Clock();
 const session:any={clock,audio,files:()=>bytes,ensureFile:async()=>{},onLog(){},instanceFrom:()=>null,textEncoding:()=> 'macintosh',puppetParams:new Map(),rng:()=>0.25,ambientRng:()=>0.25};const c=new PuppetController(session);if(!await c.openPuppetFile('test.pup'))throw Error(path);
 const puppet=c.puppet!,entry:any={path,defaultStance:puppet.defaultStance,defaultPose:puppet.defaultPose,lines:[]};let now=100;
 for(const line of puppet.pup.dialogue.values()){
  clock.advance(now);puppet.interrupted=false;puppet.voiceQueue=[];c.puppetBase(line.ident);const base=c.puppetFrame();const before=audio.calls.length,pending=c.puppetSpeak(line.ident);
  const first=c.puppetFrame(),subtitle=puppet.subtitle,stance=puppet.stanceIdx;clock.advance(now+50);const at50=c.puppetFrame();c.skipLine();await pending;
  entry.lines.push({ident:line.ident,base,first,at50,last:c.puppetFrame(),subtitle,stance,seconds:audio.calls.length>before?audio.calls.at(-1)!.seconds:null});now+=100;
 }
 out.push(entry);
}
writeFileSync(process.argv[3],JSON.stringify(out));console.log('Puppet reference: '+out.length+' characters, '+out.reduce((n,p)=>n+p.lines.length,0)+' dialogue lines');
