/* GPL-3.0-or-later. The Go codec ABI borrows checked QuickJS buffers. */
#ifndef TITANIC_CODECS_H
#define TITANIC_CODECS_H
#include "pixels.h"
#ifdef TITANIC_GO_CODECS
#ifdef __cplusplus
extern "C" {
#endif
int64_t taoot_decode_frame(uint8_t *,int64_t,uint8_t *,uint8_t *,int64_t,int,char *,int64_t);
int taoot_decode_audio(uint8_t *,int64_t,float *,int64_t,int,char *,int64_t);
int taoot_stereo_pcm(float *,int64_t,uint8_t *,double,double);
#ifdef __cplusplus
}
#endif
static JSValue titanic_decode_frame(JSContext *ctx,JSValueConst self,int argc,JSValueConst *argv) {
 (void)self;
 if(argc!=4)return JS_ThrowTypeError(ctx,"Frame decoder needs four arguments");
 TitanicPixels v[3]={{JS_UNDEFINED,NULL,0},{JS_UNDEFINED,NULL,0},{JS_UNDEFINED,NULL,0}};
 JSValue result=JS_EXCEPTION;int valid=1;
 for(int i=0;i<3;i++)if(!titanic_pixel_view(ctx,argv[i],&v[i])){valid=0;break;}
 if(valid){
  char error[256]={0};
  if(v[1].length!=v[2].length)result=JS_ThrowRangeError(ctx,"Unequal frame buffers");
  else {
   int64_t offset=taoot_decode_frame(v[0].data,v[0].length,v[1].data,v[2].data,v[1].length,JS_ToBool(ctx,argv[3]),error,sizeof(error));
   result=offset<-1?JS_ThrowInternalError(ctx,"%s",error):JS_NewInt64(ctx,offset);
  }
 }
 for(int i=0;i<3;i++)JS_FreeValue(ctx,v[i].buffer);return result;
}
static JSValue titanic_decode_audio(JSContext *ctx,JSValueConst self,int argc,JSValueConst *argv) {
 (void)self;
 if(argc!=3)return JS_ThrowTypeError(ctx,"Audio decoder needs three arguments");
 TitanicPixels v[2]={{JS_UNDEFINED,NULL,0},{JS_UNDEFINED,NULL,0}};JSValue result=JS_EXCEPTION;
 if(titanic_pixel_view(ctx,argv[0],&v[0])&&titanic_typed_view(ctx,argv[1],&v[1],4)){
  char error[256]={0};
  int status=taoot_decode_audio(v[0].data,v[0].length,(float*)v[1].data,v[1].length/4,JS_ToBool(ctx,argv[2]),error,sizeof(error));
  result=status?JS_ThrowInternalError(ctx,"%s",error):JS_UNDEFINED;
 }
 for(int i=0;i<2;i++)JS_FreeValue(ctx,v[i].buffer);return result;
}
static JSValue titanic_stereo_pcm(JSContext *ctx,JSValueConst self,int argc,JSValueConst *argv) {
 (void)self;
 if(argc!=4)return JS_ThrowTypeError(ctx,"Stereo conversion needs four arguments");
 TitanicPixels v[2]={{JS_UNDEFINED,NULL,0},{JS_UNDEFINED,NULL,0}};JSValue result=JS_EXCEPTION;double gain=0,pan=0;
 if(titanic_typed_view(ctx,argv[0],&v[0],4)&&titanic_pixel_view(ctx,argv[1],&v[1])&&JS_ToFloat64(ctx,&gain,argv[2])>=0&&JS_ToFloat64(ctx,&pan,argv[3])>=0){
  if(v[0].length!=v[1].length)result=JS_ThrowRangeError(ctx,"Stereo buffer size mismatch");
  else result=taoot_stereo_pcm((float*)v[0].data,v[0].length/4,v[1].data,gain,pan)?JS_ThrowInternalError(ctx,"Stereo conversion failed"):JS_UNDEFINED;
 }
 for(int i=0;i<2;i++)JS_FreeValue(ctx,v[i].buffer);return result;
}
static void titanic_install_codecs(JSContext *ctx,JSValueConst global){
 JS_SetPropertyStr(ctx,global,"__decodeFrame",JS_NewCFunction(ctx,titanic_decode_frame,"__decodeFrame",4));
 JS_SetPropertyStr(ctx,global,"__decodeAudio",JS_NewCFunction(ctx,titanic_decode_audio,"__decodeAudio",3));
 JS_SetPropertyStr(ctx,global,"__stereoPCM",JS_NewCFunction(ctx,titanic_stereo_pcm,"__stereoPCM",4));
}
#else
static void titanic_install_codecs(JSContext *ctx,JSValueConst global){(void)ctx;(void)global;}
#endif
#endif
