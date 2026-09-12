// Temporary migration oracle with synthetic palette bytes only.
import {writeFileSync} from 'node:fs';import {createHash} from 'node:crypto';
import {displayPalette,screenGammas,screenGammaGeneration,setScreenGamma,stepScreenGamma,resetScreenGamma,ALL_CHANNELS} from '../vendor/dreamrefactory/engine/src/web/screen-gamma';
import {dimPalette} from '../vendor/dreamrefactory/engine/src/web/screen-director';
const palette=new Uint8ClampedArray(Array.from({length:1024},(_,i)=>(i*17+19)&255));const out=[];
for(let i=0;i<140;i++){const channels=[i%3===0,i%3===1,i%3===2] as [boolean,boolean,boolean];let op='up',value=0;
 if(i===0){op='reset';resetScreenGamma()}else if(i%17===0){op='set';value=[.1,.3,.65,1,1.6,2][Math.floor(i/17)%6];setScreenGamma(value,channels)}else{op=i%7<4?'up':'down';stepScreenGamma(op==='up',channels)}
 const dim={lo:i%5-1,hi:250+i%7,amt:i%2?i*2.5:-7},bytes=displayPalette(dimPalette(palette,dim));
 out.push({op,value,channels,dim,gammas:[...screenGammas()],generation:screenGammaGeneration(),hash:createHash('sha256').update(bytes).digest('hex')});
}
writeFileSync(process.argv[2],'[\n'+out.map(x=>JSON.stringify(x)).join(',\n')+'\n]\n');
