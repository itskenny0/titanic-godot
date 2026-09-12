// Temporary migration oracle, synthetic pixels and text only.
import {writeFileSync} from 'node:fs';import {createHash} from 'node:crypto';
import {ScreenPresenter} from '../vendor/dreamrefactory/engine/src/web/screen-presenter';
import {DrawSignature} from '../vendor/dreamrefactory/engine/src/runtime/signature';
import {wrapText} from '../vendor/dreamrefactory/engine/src/web/fonts';
const screen=new ScreenPresenter({width:8,height:6});
const ctx:any={canvas:{width:8,height:6},font:'12px Arial',fillStyle:'#fff',strokeStyle:'#fff',lineWidth:1,commands:[],stack:[],version:0,
 putImageData(){this.commands=[];this.version++},createImageData(w:number,h:number){return {data:new Uint8ClampedArray(w*h*4)}},
 save(){this.stack.push([this.font,this.fillStyle,this.strokeStyle,this.lineWidth])},restore(){[this.font,this.fillStyle,this.strokeStyle,this.lineWidth]=this.stack.pop()},
 fillText(text:string,x:number,y:number){this.commands.push({op:'text',text,x,y,w:0,h:0,font:this.font,color:this.fillStyle})},
 fillRect(x:number,y:number,w:number,h:number){this.commands.push({op:'rect',x,y,w,h,color:this.fillStyle})}};
screen.clearFrame();const src=new Uint8ClampedArray(Array.from({length:4*3*4},(_,i)=>i*5&255));screen.blitAt(src,4,3,2,1);screen.frameValid=true;
const sig=new DrawSignature();sig.reset().num(17);const repaint=[];for(let i=0;i<125;i++){const yes=screen.shouldPaint(sig);repaint.push(yes);if(yes)screen.blit(ctx)}
const fades=[];for(const level of [.5,.0625,.0005,.1005,1.1,-1,0,.666666]){screen.blit(ctx);screen.drawTextOverlay(ctx,[{text:'Test',x:1,y:3,color:0,size:12},{text:'Café',x:2,y:4,color:3,size:16}]);screen.applyFadeExcept(ctx,level,{x:2,y:1,w:4,h:3});fades.push({level,commands:ctx.commands.map((x:any)=>({...x}))})}
const wraps=['A short line','one two three four','  leading   spaces ','日本語の字幕、正しく改行。','word 😀 next'].map(text=>({text,lines:wrapText(text,55,s=>s.length*7)}));
writeFileSync(process.argv[2],JSON.stringify({pixels:createHash('sha256').update(screen.frame).digest('hex'),repaint,fades,wraps},null,2)+'\n');
