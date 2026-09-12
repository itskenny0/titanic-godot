// Temporary migration oracle. Keep output alongside owned data, outside Git.
import {readFileSync,writeFileSync} from 'node:fs';
import {extname} from 'node:path';
import {createHash} from 'node:crypto';
import {readShpFile} from '../vendor/dreamrefactory/engine/src/df/shp';
import {readCstFile} from '../vendor/dreamrefactory/engine/src/df/cst';
import {PropRuntime,frameIndexForDegree,degVariantFrames,playSequence} from '../vendor/dreamrefactory/engine/src/runtime/props';
import {ActorRuntime} from '../vendor/dreamrefactory/engine/src/runtime/actors';
import {projectPoint,bearing} from '../vendor/dreamrefactory/engine/src/runtime/geometry';
import {DrawSignature} from '../vendor/dreamrefactory/engine/src/runtime/signature';
const hash=(b:Uint8Array)=>createHash('sha256').update(b).digest('hex');
const cam={x:0,y:0,z:0,deg:0,f:96,cx:32,cy:24,clipW:60,clipH:44};
const palette=Uint8ClampedArray.from({length:1024},(_,i)=>(i*71+13)%256);
const z=Uint8Array.from({length:64*48},(_,i)=>i%7===0?0:255),occ={z,w:64,h:48,scale:16,levels:256,groundBias:3};
const rect=(r:any)=>r?{x:r.x,y:r.y,w:r.w,h:r.h}:null;
const output:any={props:[],actors:[],geometry:[],signature:[]};
for(let deg=0;deg<256;deg++)for(const xyz of [[96,12,7],[-96,-12,-7],[0,0,0],[1234567,7654321,-222],[0.25,0.75,-0.5]]) {
 const camera={...cam,deg};output.geometry.push({cam:camera,xyz,projection:projectPoint(camera,...xyz as [number,number,number]),bearing:bearing(xyz[0],xyz[1])});
}
const sig=new DrawSignature();for(const v of [0,1,-1,2147483648,-0.5,Math.PI,'Titanic','𐀀é',true,false]){if(typeof v==='string')sig.str(v);else if(typeof v==='boolean')sig.bool(v);else sig.num(v);output.signature.push([sig.lo,sig.hi]);}
for(const {path}of JSON.parse(readFileSync(process.argv[2],'utf8'))){
 const kind=extname(path).toLowerCase();if(!['.shp','.cst'].includes(kind))continue;
 const bytes=new Uint8Array(readFileSync(path));
 if(kind==='.shp'){
  const shp=readShpFile(bytes),runtime=new PropRuntime();runtime.addShop('test',shp);
  for(const group of shp.groups){const p=runtime.get(group.name)!;p.visible=true;
   for(const st of group.states){if(!st.frames.length)continue;p.stateName=st.identifier.toLowerCase();
    for(let mode=0;mode<4;mode++){
     p.deg=[-0.5,63.5,128,257][mode];p.frameIdx=mode%st.frames.length;p.frameOrder=mode===3?playSequence(st,degVariantFrames(st,p.deg)):null;
     p.worldSpace=mode!==0;p.directional=mode===2;p.worldX=96;p.worldY=4;p.worldZ=2;p.scale=62.5;p.zclip=mode*5;
     const f=p.shop.frame(p.currentFrame(st));p.anchorX=f.posXraw-Math.floor(f.width/2)+32;p.anchorY=f.posYraw-Math.floor(f.height/2)+24;
     const pixels=new Uint8ClampedArray(64*48*4).fill(17),hits=new Uint8Array(64*48);
     runtime.composite(pixels,64,48,palette,-Infinity,cam,false,occ);
     for(let y=0;y<48;y++)for(let x=0;x<64;x++)hits[y*64+x]=runtime.propAt(x,y,cam,false,occ)?1:0;
     const projected=projectPoint(cam,p.worldX,p.worldY,p.worldZ)!;
     output.props.push({path,group:group.name,state:p.stateName,mode,frame:p.currentFrameIdx(st),variant:degVariantFrames(st,p.deg),order:p.frameOrder,rect:rect(p.worldSpace?(runtime as any).worldRect(p,projected,cam):p.screenRect()),pixels:hash(pixels),hits:hash(hits)});
    }
   }p.visible=false;
  }
 }else{
  const cst=readCstFile(bytes),runtime=new ActorRuntime();runtime.addCast('test',cst);
  for(const member of cst.members){const a=runtime.get(member.name)!;a.visible=true;
   for(const pose of member.poses){a.poseName=pose.name;
    for(let mode=0;mode<4;mode++){
     a.step=pose.play.length?mode%pose.play.length:0;a.deg=[-0.5,63.5,128,257][mode];a.worldSpace=mode!==0;a.worldX=96;a.worldY=4;a.worldZ=2;a.scale=mode===0?0:62.5;a.zclip=mode*5;
     const f=(runtime as any).frameFor(a,null);a.anchorX=f?f.posXraw-Math.floor(f.width/2)+32:32;a.anchorY=f?f.posYraw-Math.floor(f.height/2)+24:24;
     const pixels=new Uint8ClampedArray(64*48*4).fill(17),hits=new Uint8Array(64*48);
     runtime.composite(pixels,64,48,palette,cam,occ);runtime.compositeScreen(pixels,64,48,palette);
     for(let y=0;y<48;y++)for(let x=0;x<64;x++)hits[y*64+x]=runtime.actorAt(x,y,cam,occ)?1:0;
     output.actors.push({path,member:member.name,pose:pose.name,mode,rect:rect(a.worldSpace?runtime.rect(a,projectPoint(cam,a.worldX,a.worldY,a.worldZ)!,cam):runtime.screenRect(a)),onScreen:runtime.onScreen(a,cam),pixels:hash(pixels),hits:hash(hits)});
    }
   }a.visible=false;
  }
 }
}
writeFileSync(process.argv[3],JSON.stringify(output));console.log(`Sprite runtime reference: ${output.props.length} prop cases, ${output.actors.length} actor cases`);
