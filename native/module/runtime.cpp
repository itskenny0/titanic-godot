#include "runtime.h"
#include "pixels.h"
#include "codecs.h"
#include "memory.h"
#include <cstring>
#if VERSION_MAJOR >= 4
#include "core/variant/array.h"
#else
#include "core/array.h"
#endif
void DreamRuntime::_bind_methods() {
 ClassDB::bind_method(D_METHOD("initialize", "owner", "code"), &DreamRuntime::initialize);
 ClassDB::bind_method(D_METHOD("execute", "code"), &DreamRuntime::execute);
 ClassDB::bind_method(D_METHOD("query", "code"), &DreamRuntime::query);
 ClassDB::bind_method(D_METHOD("buffer", "code"), &DreamRuntime::buffer);
}
DreamRuntime::DreamRuntime() {
 rt=JS_NewRuntime(); JS_SetMemoryLimit(rt,TITANIC_HEAP_LIMIT); JS_SetMaxStackSize(rt,2*1024*1024);
 ctx=JS_NewContext(rt); JS_SetContextOpaque(ctx,this);
 JSValue global=JS_GetGlobalObject(ctx);
 titanic_install_codecs(ctx,global);
 JS_SetPropertyStr(ctx,global,"__runtimeMemory",JS_NewCFunction(ctx,titanic_memory_stats,"__runtimeMemory",0));
 JS_SetPropertyStr(ctx,global,"__indexedRGBA",JS_NewCFunction(ctx,titanic_indexed_rgba,"__indexedRGBA",4));
 JS_SetPropertyStr(ctx,global,"__native",JS_NewCFunction(ctx,native_call,"__native",3)); JS_FreeValue(ctx,global);
}
DreamRuntime::~DreamRuntime() { JS_FreeContext(ctx); JS_FreeRuntime(rt); }
void DreamRuntime::exception() {
 JSValue value=JS_GetException(ctx), stack=JS_GetPropertyStr(ctx,value,"stack");
 const char *text=JS_ToCString(ctx,value), *trace=JS_IsUndefined(stack)?nullptr:JS_ToCString(ctx,stack);
 last_error=String::utf8(text?text:"JavaScript error")+"\n"+String::utf8(trace?trace:"");
 ERR_PRINT(last_error); JS_FreeCString(ctx,text); JS_FreeCString(ctx,trace); JS_FreeValue(ctx,value); JS_FreeValue(ctx,stack);
}
JSValue DreamRuntime::eval(const String &code) {
 last_error=String(); titanic_prepare_memory(rt);
 CharString text=code.utf8(); JSValue v=JS_Eval(ctx,text.get_data(),text.length(),"engine.js",JS_EVAL_TYPE_GLOBAL);
 if(JS_IsException(v))exception();return v;
}
void DreamRuntime::pump() { JSContext *pending;for(int i=0;i<4096&&JS_IsJobPending(rt);i++) if(JS_ExecutePendingJob(rt,&pending)<0){exception();break;} }
String DreamRuntime::initialize(Object *owner,const String &code) {host=owner;return execute(code);}
String DreamRuntime::execute(const String &code) { JSValue v=eval(code);JS_FreeValue(ctx,v);pump();return last_error; }
String DreamRuntime::query(const String &code) {
 JSValue v=eval(code); const char *text=JS_IsException(v)?nullptr:JS_ToCString(ctx,v);
 String out=String::utf8(text?text:"");JS_FreeCString(ctx,text);JS_FreeValue(ctx,v);return out;
}
Bytes DreamRuntime::buffer(const String &code) {
 JSValue v=eval(code);Bytes out;
 if(!JS_IsException(v)&&!JS_IsNull(v)&&!JS_IsUndefined(v)) {
  size_t size=0;uint8_t *data=JS_GetArrayBuffer(ctx,&size,v);
  if(data){out.resize(size);
#if VERSION_MAJOR >= 4
   memcpy(out.ptrw(),data,size);
#else
   auto w=out.write();memcpy(w.ptr(),data,size);
#endif
  }else exception();
 }JS_FreeValue(ctx,v);return out;
}
JSValue DreamRuntime::native_call(JSContext *ctx,JSValueConst self,int argc,JSValueConst *argv) {
 DreamRuntime *r=static_cast<DreamRuntime*>(JS_GetContextOpaque(ctx));
 if(!r->host||argc<2)return JS_ThrowTypeError(ctx,"Invalid native call");
 const char *method=JS_ToCString(ctx,argv[0]), *json=JS_ToCString(ctx,argv[1]);
 if(!method||!json){JS_FreeCString(ctx,method);JS_FreeCString(ctx,json);return JS_EXCEPTION;}
 String m=String::utf8(method),j=String::utf8(json);JS_FreeCString(ctx,method);JS_FreeCString(ctx,json);
 Variant binary;
 if(argc>2&&!JS_IsUndefined(argv[2])) {
  size_t size=0;uint8_t *data=JS_GetArrayBuffer(ctx,&size,argv[2]);if(!data)return JS_EXCEPTION;
  Bytes b;b.resize(size);
#if VERSION_MAJOR >= 4
  memcpy(b.ptrw(),data,size);
#else
  {auto w=b.write();memcpy(w.ptr(),data,size);}
#endif
  binary=b;
 }
 // Godot 3 variadic call treats a nil argument as the end of the list.
 // callv retains the required third argument even when there is no payload.
 Array arguments;arguments.append(m);arguments.append(j);arguments.append(binary);
 Variant result=r->host->callv("bridge_call",arguments);
#if VERSION_MAJOR >= 4
 if(result.get_type()==Variant::PACKED_BYTE_ARRAY){Bytes b=result;return JS_NewArrayBufferCopy(ctx,b.ptr(),b.size());}
#else
 if(result.get_type()==Variant::POOL_BYTE_ARRAY){Bytes b=result;auto rd=b.read();return JS_NewArrayBufferCopy(ctx,rd.ptr(),b.size());}
#endif
 if(result.get_type()==Variant::STRING){CharString s=String(result).utf8();return JS_NewStringLen(ctx,s.get_data(),s.length());}
 return JS_NULL;
}
