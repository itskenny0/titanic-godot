// Temporary migration oracle. Frame hashes and authored camera positions stay local.
import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {readSetFile} from '../vendor/dreamrefactory/engine/src/df/set';
import {RingCache} from '../vendor/dreamrefactory/engine/src/web/ring-cache';
const hash=(b:Uint8Array)=>createHash('sha256').update(b).digest('hex');
const paths=JSON.parse(readFileSync(process.argv[2],'utf8')).filter((x:any)=>x.path.toLowerCase().endsWith('.set')),output=[];
for(const {path}of paths){const set=readSetFile(new Uint8Array(readFileSync(path))),cache=new RingCache(set),registers=[...set.scenes.flatMap(s=>s.turns),...set.transitions.flatMap(t=>t.frameRegisters)];const visits=[];
 for(const index of [...registers.keys(),...registers.keys()].reverse()){
  const reg=registers[index],needed=cache.needsDecode(reg.frames),frames=cache.ensure(reg.frames);
  visits.push({index,needed,bytes:(cache as any).decodedBytes,frames:[...frames].map(([loc,f])=>({loc,width:f.width,height:f.height,pixels:hash(f.pixels),z:f.z?hash(f.z):'',camera:f.cam}))});
 }
 output.push({path,visits});
}
writeFileSync(process.argv[3],JSON.stringify(output));console.log(`Ring reference: ${output.length} sets, ${output.reduce((n,s)=>n+s.visits.length,0)} ring visits`);
