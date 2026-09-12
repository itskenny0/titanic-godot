import {readFileSync,writeFileSync} from 'node:fs';
import {basename,extname} from 'node:path';
import {createHash} from 'node:crypto';
import {AudioLibrary} from '../vendor/dreamrefactory/engine/src/runtime/audio';
const fingerprint=(a:any)=>a?{rate:a.sampleRate,samples:a.samples.length,pcm:createHash('sha256').update(new Uint8Array(a.samples.buffer)).digest('hex')}:null;
const out=[];
for(const {path}of JSON.parse(readFileSync(process.argv[2],'utf8'))){if(!['.trk','.sfx','.11k'].includes(extname(path).toLowerCase()))continue;const lib=new AudioLibrary(),name=basename(path);if(!lib.openBank(name,new Uint8Array(readFileSync(path))))throw Error(path);out.push({path,name,trackName:lib.trackNameOf(name),sounds:lib.soundNames(name).map(name=>({name,audio:fingerprint(lib.sound(name))})),theme:fingerprint(lib.theme(name)),loopTable:lib.loopTable(name)});}
writeFileSync(process.argv[3],JSON.stringify(out));console.log('Audio library reference: '+out.length+' banks');
