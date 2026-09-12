import {readFileSync,writeFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {readContainerFile} from '../vendor/dreamrefactory/engine/src/df/container';
import {decodeScript} from '../vendor/dreamrefactory/engine/src/df/script';
import {parseScript} from '../vendor/dreamrefactory/engine/src/runtime/parser';
function canonical(v:any):any {
 if(Array.isArray(v))return v.map(canonical);
 if(v&&typeof v==='object')return Object.fromEntries(Object.keys(v).sort().filter(k=>v[k]!==undefined).map(k=>[k,canonical(v[k])]));
 return v;
}
const hash=(v:any)=>createHash('sha256').update(JSON.stringify(canonical(v))).digest('hex');
const corpus=[...JSON.parse(readFileSync(process.argv[2],'utf8')),...process.argv.slice(4).map(path=>({path}))],results=[];
let scripts=0,parsed=0;
for(const {path} of corpus){
 const file=readContainerFile(new Uint8Array(readFileSync(path))),entry:any={path,scripts:[]};
 for(let loc=0;loc<file.containers.length;loc++){
  let tokens;try{tokens=decodeScript(file.containers[loc].data);}catch{continue;}
  if(!tokens.length)continue;
  let ast='';try{const s=parseScript(tokens);ast=hash({codes:[...s.codes],topLevel:s.topLevel});parsed++;}catch{}
  entry.scripts.push({loc,tokens:hash(tokens),ast});scripts++;
 }
 if(entry.scripts.length)results.push(entry);
}
writeFileSync(process.argv[3],JSON.stringify(results));
console.log(JSON.stringify({files:results.length,scripts,parsed}));
