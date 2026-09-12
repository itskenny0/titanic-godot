/* GPL-3.0. Checked, zero-copy palette expansion for QuickJS typed arrays. */
#ifndef TITANIC_PIXELS_H
#define TITANIC_PIXELS_H
#include "quickjs.h"
#include <stdint.h>
#include <string.h>
typedef struct { JSValue buffer; uint8_t *data; size_t length; } TitanicPixels;
static int titanic_pixel_view(JSContext *ctx, JSValueConst value, TitanicPixels *view) {
 size_t offset=0, element=0, total=0;
 view->buffer=JS_GetTypedArrayBuffer(ctx,value,&offset,&view->length,&element);
 if(JS_IsException(view->buffer))return 0;
 if(element!=1){JS_ThrowTypeError(ctx,"Pixel data must use byte arrays");return 0;}
 uint8_t *data=JS_GetArrayBuffer(ctx,&total,view->buffer);
 if(!data||offset>total||view->length>total-offset){JS_ThrowTypeError(ctx,"Invalid pixel buffer");return 0;}
 view->data=data+offset;return 1;
}
static JSValue titanic_indexed_rgba(JSContext *ctx, JSValueConst self, int argc, JSValueConst *argv) {
 (void)self;
 if(argc!=4)return JS_ThrowTypeError(ctx,"Palette expansion needs four arguments");
 uint64_t count=0;
 if(JS_ToIndex(ctx,&count,argv[3])<0)return JS_EXCEPTION;
 if(count>16777216)return JS_ThrowRangeError(ctx,"Pixel count exceeds limit");
 TitanicPixels views[3]={{JS_UNDEFINED,NULL,0},{JS_UNDEFINED,NULL,0},{JS_UNDEFINED,NULL,0}};
 int valid=1;
 for(int i=0;i<3;i++)if(!titanic_pixel_view(ctx,argv[i],&views[i])){valid=0;break;}
 JSValue result=JS_EXCEPTION;
 if(valid){
  if(views[0].length<count||views[1].length<1024||views[2].length<count*4)
   result=JS_ThrowRangeError(ctx,"Pixel buffer is too short");
  else {
   uint8_t palette[1024];
   memcpy(palette,views[1].data,sizeof(palette));
   for(int i=3;i<1024;i+=4)palette[i]=255;
   for(size_t i=0;i<(size_t)count;i++)memcpy(views[2].data+i*4,palette+views[0].data[i]*4,4);
   result=JS_DupValue(ctx,argv[2]);
  }
 }
 for(int i=0;i<3;i++)JS_FreeValue(ctx,views[i].buffer);
 return result;
}
#endif
