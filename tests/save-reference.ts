// Temporary migration oracle. Save contents and generated output stay local.
import {readFileSync,writeFileSync,readdirSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {createHash} from 'node:crypto';
import {createNeutralSaveTemplate} from '../engine/restoration/neutral-save';
import {parseRestoredSave} from '../engine/restoration/save-extension';
import {readSaveFile,writeSaveFile} from '../vendor/dreamrefactory/engine/src/df/savegame';
const hash=(b:Uint8Array)=>createHash('sha256').update(b).digest('hex');
const normalized=(s:any)=>{const {raw,index,...state}=s;return {...state,numGlobals:Object.fromEntries(s.numGlobals),strGlobals:Object.fromEntries(s.strGlobals)}};
const pstr=(b:Uint8Array,at:number,s:string)=>{b[at]=s.length;for(let i=0;i<s.length;i++)b[at+1+i]=s.charCodeAt(i)};
const raw=readSaveFile(createNeutralSaveTemplate());
pstr(raw.containers[0].data,256,'TitanicCD2');pstr(raw.containers[1].data,596,'hallc');pstr(raw.containers[1].data,612,'scene2');pstr(raw.containers[1].data,628,'view7');
new DataView(raw.containers[1].data.buffer).setUint32(442,98765,true);
const vars=new Uint8Array(32*13),vv=new DataView(vars.buffer),pool=new Uint8Array(96);pstr(pool,0,'23:10');
for(let i=0;i<12;i++){vv.setUint32(i*32+20,0x12345678,true);vv.setUint16(i*32+24,i===0?3:i%2?4:2,true);vv.setInt32(i*32+26,i===0?0:i%2?-i*1000:1,true);if(i>0)pstr(vars,i*32+8,i===1?'clock':'var'+i)}
raw.containers[7].data=vars;raw.containers[8].data=pool;
for(const [ci,stride,offset]of [[2,160,80],[4,158,78]]){const b=new Uint8Array(stride*2),dv=new DataView(b.buffer);for(let i=0;i<2;i++){const at=i*stride;dv.setUint16(at,1,true);dv.setInt16(at+18,1,true);for(let j=20;j<78;j+=2)dv.setInt16(at+j,j*(i?-1:1),true);pstr(b,at+offset,'object'+i);pstr(b,at+offset+16,'hallc');pstr(b,at+offset+32,'anchor');pstr(b,at+offset+48,'idle');pstr(b,at+offset+64,'inventory')}raw.containers[ci].data=b;}
const casts=new Uint8Array(56);pstr(casts,12,'gang.cst');pstr(casts,40,'extras.cst');raw.containers[3].data=casts;
const loops=raw.containers[9].data,lv=new DataView(loops.buffer);lv.setUint16(0,1,true);lv.setUint16(4,3,true);lv.setUint32(6,99,true);pstr(loops,10,'scene2');pstr(loops,26,'calctime');
const crickets=raw.containers[10].data,cv=new DataView(crickets.buffer);cv.setUint16(0,1,true);cv.setInt16(4,-123,true);cv.setInt16(6,234,true);cv.setUint32(8,1234,true);cv.setUint32(12,50,true);cv.setInt32(16,-1,true);cv.setUint32(20,47,true);pstr(crickets,42,'hallc');pstr(crickets,58,'rumble');
const walks=raw.containers[11].data,wv=new DataView(walks.buffer);wv.setUint16(0,1,true);wv.setInt16(4,3,true);wv.setInt16(8,-1,true);wv.setInt16(10,64,true);wv.setInt16(12,123,true);wv.setInt16(14,-234,true);wv.setInt16(16,12,true);wv.setUint32(18,1,true);wv.setInt32(22,9,true);pstr(walks,46,'object0');pstr(walks,62,'anchor2');
const route=new Uint8Array(44),rv=new DataView(route.buffer);rv.setUint32(0,30,true);rv.setUint32(8,3,true);for(let i=0;i<3;i++){rv.setInt16(20+i*8,i*12,true);rv.setInt16(22+i*8,-i*2,true);rv.setUint16(26+i*8,i*10,true)}
raw.containers.push({id:12,data:route});
// Insert one track with a playing record and repeated entries in its loop order.
const tracks=new Uint8Array(40),tv=new DataView(tracks.buffer);tv.setInt16(6,1,true);tv.setInt16(8,2,true);pstr(tracks,22,'theme.trk');raw.containers[6].data=tracks;
const playing=new Uint8Array(104);new DataView(playing.buffer).setUint16(4,123,true);pstr(playing,8,'part1');const looping=new Uint8Array(208);looping.set(playing);looping.set(playing,104);
raw.containers.splice(7,0,{id:7,data:new Uint8Array()},{id:8,data:playing},{id:9,data:looping});raw.containers.forEach((c,i)=>c.id=i);
const synthetic=writeSaveFile(raw);writeFileSync('.build/synthetic-legacy-save.ti',synthetic);
const paths=['.build/synthetic-legacy-save.ti'];
function walk(dir:string){for(const e of readdirSync(dir,{withFileTypes:true})){const p=join(dir,e.name);if(e.isDirectory())walk(p);else if(e.name.endsWith('.ti'))paths.push(p)}}
walk('.build/android-memory-backup');paths.push('.build/go-port-integration/Retanic/Saves/retanic-integration.ti');
const out={neutral:hash(createNeutralSaveTemplate()),saves:paths.map(path=>{const b=new Uint8Array(readFileSync(path));return {path:resolve(path),state:normalized(parseRestoredSave(b)),rewrite:hash(writeSaveFile(readSaveFile(b)))}})};
writeFileSync(process.argv[2],JSON.stringify(out));console.log('Save reference: '+out.saves.length+' saves');
