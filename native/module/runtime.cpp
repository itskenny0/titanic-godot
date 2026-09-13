#include "runtime.h"
#include <cstdlib>
#include <cstring>
#if VERSION_MAJOR >= 4
#include "core/variant/array.h"
#else
#include "core/array.h"
#endif
void DreamRuntime::_bind_methods() {
 ClassDB::bind_method(D_METHOD("initialize","owner"),&DreamRuntime::initialize);
 ClassDB::bind_method(D_METHOD("execute","method","args"),&DreamRuntime::execute,DEFVAL("{}"));
 ClassDB::bind_method(D_METHOD("query","method","args"),&DreamRuntime::query,DEFVAL("{}"));
 ClassDB::bind_method(D_METHOD("buffer","method","args"),&DreamRuntime::buffer,DEFVAL("{}"));
}
DreamRuntime::~DreamRuntime(){if(handle)taoot_player_close(handle);}
String DreamRuntime::initialize(Object *owner) {
 if(handle)taoot_player_close(handle);host=owner;handle=taoot_player_new((uintptr_t)this,(void*)platform_call);
 return handle?String():String("Could not initialize Go runtime");
}
TaootResult DreamRuntime::call(const String &method,const String &args) {
 TaootResult out={};CharString m=method.utf8(),a=args.utf8();
 taoot_player_call(handle,(char*)m.get_data(),(char*)a.get_data(),&out);return out;
}
String DreamRuntime::execute(const String &method,const String &args) {
 TaootResult out=call(method,args);String error=out.kind<0?String::utf8((char*)out.data,(int)out.size):String();free(out.data);return error;
}
String DreamRuntime::query(const String &method,const String &args) {
 TaootResult out=call(method,args);String result=out.kind==1?String::utf8((char*)out.data,(int)out.size):String("null");free(out.data);return result;
}
Bytes DreamRuntime::buffer(const String &method,const String &args) {
 TaootResult result=call(method,args);Bytes out;
 if(result.kind==2 && result.size>0){out.resize((int)result.size);
#if VERSION_MAJOR >= 4
  memcpy(out.ptrw(),result.data,(size_t)result.size);
#else
  auto w=out.write();memcpy(w.ptr(),result.data,(size_t)result.size);
#endif
 }
 free(result.data);return out;
}
static void copy_result(TaootResult *out,const void *data,int64_t size,int kind) {
 out->data=size?(uint8_t*)malloc((size_t)size):nullptr;out->size=size;out->kind=kind;if(size)memcpy(out->data,data,(size_t)size);
}
void DreamRuntime::platform_call(uintptr_t owner,const char *method,const char *json,const uint8_t *data,int64_t size,TaootResult *out) {
 auto r=(DreamRuntime*)owner;if(!r->host){const char *e="Missing platform owner";copy_result(out,e,strlen(e),-1);return;}
 Variant binary;
 if(data){Bytes b;b.resize((int)size);
#if VERSION_MAJOR >= 4
  memcpy(b.ptrw(),data,(size_t)size);
#else
  {auto w=b.write();memcpy(w.ptr(),data,(size_t)size);}
#endif
  binary=b;
 }
 Array arguments;arguments.append(String::utf8(method));arguments.append(String::utf8(json));arguments.append(binary);
 Variant result=r->host->callv("bridge_call",arguments);
#if VERSION_MAJOR >= 4
 if(result.get_type()==Variant::PACKED_BYTE_ARRAY){Bytes b=result;copy_result(out,b.ptr(),b.size(),2);}
#else
 if(result.get_type()==Variant::POOL_BYTE_ARRAY){Bytes b=result;auto rd=b.read();copy_result(out,rd.ptr(),b.size(),2);}
#endif
 else if(result.get_type()==Variant::STRING){CharString s=String(result).utf8();copy_result(out,s.get_data(),s.length(),1);}
}
