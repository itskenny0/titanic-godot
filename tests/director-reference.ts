// Temporary migration oracle. This fixture contains only synthetic pixels/text.
import {writeFileSync} from 'node:fs';import {createHash} from 'node:crypto';
import {GameSession} from '../vendor/dreamrefactory/engine/src/runtime/session';
import {NullAudioSink} from '../vendor/dreamrefactory/engine/src/runtime/audio';
import {ScreenDirector} from '../vendor/dreamrefactory/engine/src/web/screen-director';
import {setScreenGamma,displayPalette,screenGammaGeneration} from '../vendor/dreamrefactory/engine/src/web/screen-gamma';
import {paletteToRGBA} from '../vendor/dreamrefactory/engine/src/df/image';
const hash=(b:Uint8Array|Uint8ClampedArray)=>createHash('sha256').update(b).digest('hex');
const inputs:any[]=[];
for(const flat of ['none','pattern','matte'])for(const room of [false,true])for(const showing of [false,true])inputs.push({flat,room,showing});
for(const owner of ['held','faded','puppet','movie'])for(const flat of ['none','pattern'])inputs.push({flat,room:true,showing:true,owner,fade:.625,subtitle:owner==='puppet'});
for(const dir of ['open','close','left','right','turnleft','turnright'])for(const step of [0,1,3,5])for(const span of (dir.startsWith('turn')?[.25,1]:[1]))inputs.push({flat:'pattern',room:true,showing:true,dir,step,span});
inputs.push({flat:'pattern',room:true,showing:false,gamma:1.2,dim:{lo:4,hi:80,amt:127.5}}, {flat:'pattern',room:true,showing:true,photo:true,fade:.0625}, {flat:'none',room:false,showing:false,owner:'puppet',subtitle:true});
for(const [x,y]of [[3,4],[3.25,4],[3.5,4.25],[511.75,383]])inputs.push({flat:"pattern",room:true,showing:true,xray:{x,y}});
const out=[];
for(const input of inputs){
 setScreenGamma(input.gamma??.65);const s=new GameSession(()=>undefined,new NullAudioSink()),d=new ScreenDirector(s);
 const raw=new Uint8Array(2048);for(let i=0;i<256;i++){raw[i*8+3]=i;raw[i*8+5]=i*3&255;raw[i*8+7]=i*7&255}const base=paletteToRGBA(raw,256),roomPal=displayPalette(base);
 const cur={pixels:Uint8Array.from({length:512*264},(_,i)=>i*13&255),width:512,height:264};
 const room:any={roomVersion:4,roomAnimating:false,roomFrame:()=>cur,roomPalette:()=>roomPal,roomPropPalette:()=>roomPal,bandPropPalette:()=>roomPal,roomCamera:()=>null,roomOcclusion:()=>null,roomSignature(){},drawRoomHotspots(){},pointInRoomImage:()=>true};
 if(input.room)d.setRoom(room);s.currentSetName=input.room?'room':'none';s.setVisible=input.showing;
 const flat={pixels:Uint8Array.from({length:512*384},(_,i)=>input.flat==='matte'?17:i*5&255),width:512,height:384,palette:base};
 const hidden={...flat,pixels:Uint8Array.from({length:512*384},(_,i)=>i*9+33&255)};
 (s.stageCtrl as any).flatImage=(name:string)=>name==="hidden"?hidden:input.flat==='none'?null:flat;
 if(input.xray){s.plugins.xray={aimed:true,hidden:"hidden",mask:"mask",...input.xray} as any;const get=s.propRuntime.get.bind(s.propRuntime);(s.propRuntime as any).get=(name:string)=>name==="mask"?{state:()=>({frames:[1]}),currentFrame:()=>1,shop:{frame:()=>({width:2,height:2,posXraw:0,posYraw:0,opaque:new Uint8Array([1,0,1,1])})}}:get(name)}
 if(input.dim)(d as any).stageDim=input.dim;
 for(let i=0;i<d.screen.frame.length;i++)d.screen.frame[i]=i*7&255;
 s.textOverlay=[{text:'Caption',x:7,y:23,size:12,color:2}];
 s.fade.level=input.fade??0;
 const rgba=Uint8ClampedArray.from({length:512*384*4},(_,i)=>i*11+31&255);
 if(input.owner==='held')s.fade.pendingReveal=true;
 if(input.owner==='faded')s.fade.snapshot={rgba,width:512,height:384};
 if(input.owner==='puppet'){
  (s.puppetCtrl as any).puppet={name:'test.pup',visible:true,stanceIdx:0,pup:{paletteRaw:raw,stances:[],file:{containers:[]},bandLocation:0},pose:null,anim:null,subtitle:input.subtitle?'The character speaks.':'',bevels:[{text:'Answer',id:1}],chosen:0,press:null};
 }
 if(input.owner==='movie'){
  (d.movies as any).active={fileName:'test.mov',frames:[{pixels:new Uint8Array([51,97]),width:2,height:1}],pos:0,palette:roomPal,paletteGen:screenGammaGeneration(),seg:{originX:17,originY:31,paletteRaw:raw,file:{order:'le'}}};
 }
 if(input.dir)Object.assign(s.wipe,{dir:input.dir,step:input.step,steps:6,span:input.span,settled:false,from:{rgba,width:512,height:384},to:{rgba:Uint8ClampedArray.from({length:rgba.length},(_,i)=>i*17+19&255),width:512,height:384}});
 if(input.photo)s.photoOverlay={photo:{rgba:new Uint8ClampedArray([81,82,83,255,91,92,93,255]),width:2,height:1},x:3,y:4};
 const ctx:any={canvas:{width:512,height:384},font:'12px Arial',fillStyle:'#fff',strokeStyle:'#fff',lineWidth:1,commands:[],stack:[],version:0,
 putImageData(){this.commands=[];this.version++},createImageData(w:number,h:number){return{data:new Uint8ClampedArray(w*h*4)}},save(){this.stack.push([this.font,this.fillStyle,this.strokeStyle,this.lineWidth])},restore(){[this.font,this.fillStyle,this.strokeStyle,this.lineWidth]=this.stack.pop()},measureText(text:string){return{width:text.length*7}},fillText(text:string,x:number,y:number){this.commands.push({op:'text',text,x,y,w:0,h:0,font:this.font,color:this.fillStyle})},fillRect(x:number,y:number,w:number,h:number){this.commands.push({op:'rect',x,y,w,h,color:this.fillStyle})},strokeRect(x:number,y:number,w:number,h:number){this.commands.push({op:'stroke',x,y,w,h,color:this.strokeStyle,line:this.lineWidth})}};
 d.render(ctx);const version=ctx.version;d.render(ctx);
 out.push({input,owner:d.screenOwner(),pixels:hash(d.screen.frame),valid:d.screen.frameValid,commands:ctx.commands,repaint:ctx.version!==version});
}
writeFileSync(process.argv[2],JSON.stringify(out,null,2)+'\n');console.log(`Director reference: ${out.length} screen compositions`);
