/* GPL-3.0-or-later. Godot 3 GDNative host for the unmodified DreamFactory JS engine. */
#include <gdnative_api_struct.gen.h>
#include "quickjs.h"
#include "module/pixels.h"
#include "module/codecs.h"
#include "module/memory.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

static const godot_gdnative_core_api_struct *g;
static const godot_gdnative_ext_nativescript_api_struct *ns;
typedef struct { JSRuntime *rt; JSContext *ctx; godot_variant host; char *error; } Runtime;
static godot_variant nil(void) { godot_variant v; g->godot_variant_new_nil(&v); return v; }
static godot_variant string_variant(const char *s) {
 godot_string gs; g->godot_string_new(&gs); g->godot_string_parse_utf8(&gs,s);
 godot_variant v; g->godot_variant_new_string(&v,&gs); g->godot_string_destroy(&gs); return v;
}
static void remember_error(Runtime *r) {
 JSValue e=JS_GetException(r->ctx), stack=JS_GetPropertyStr(r->ctx,e,"stack");
 const char *msg=JS_ToCString(r->ctx,e), *trace=JS_IsUndefined(stack)?NULL:JS_ToCString(r->ctx,stack);
 free(r->error); size_t n=(msg?strlen(msg):0)+(trace?strlen(trace):0)+4;
 r->error=malloc(n); snprintf(r->error,n,"%s\n%s",msg?msg:"JavaScript error",trace?trace:"");
 g->godot_print_error(r->error,"dreamREfactory","bridge.c",0);
 JS_FreeCString(r->ctx,msg); JS_FreeCString(r->ctx,trace); JS_FreeValue(r->ctx,stack); JS_FreeValue(r->ctx,e);
}
static godot_variant bytes_variant(const uint8_t *data,size_t len) {
 godot_pool_byte_array p; g->godot_pool_byte_array_new(&p); g->godot_pool_byte_array_resize(&p,(int)len);
 if(len) { godot_pool_byte_array_write_access *w=g->godot_pool_byte_array_write(&p);
 memcpy(g->godot_pool_byte_array_write_access_ptr(w),data,len); g->godot_pool_byte_array_write_access_destroy(w); }
 godot_variant v; g->godot_variant_new_pool_byte_array(&v,&p); g->godot_pool_byte_array_destroy(&p); return v;
}
/* The only JS capability is a call to our owning Godot node. No network or process APIs. */
static JSValue native_call(JSContext *ctx,JSValueConst self,int argc,JSValueConst *argv) {
 Runtime *r=JS_GetContextOpaque(ctx); (void)self;
 if(argc<2) return JS_ThrowTypeError(ctx,"native call needs method and JSON arguments");
 const char *method=JS_ToCString(ctx,argv[0]), *json=JS_ToCString(ctx,argv[1]);
 if(!method||!json) { JS_FreeCString(ctx,method); JS_FreeCString(ctx,json); return JS_EXCEPTION; }
 godot_variant args[3]={string_variant(method),string_variant(json),nil()};
 JS_FreeCString(ctx,method); JS_FreeCString(ctx,json);
 if(argc>2 && !JS_IsUndefined(argv[2])) {
  size_t n=0; uint8_t *p=JS_GetArrayBuffer(ctx,&n,argv[2]);
  if(!p) { for(int i=0;i<3;i++)g->godot_variant_destroy(&args[i]); return JS_EXCEPTION; }
  g->godot_variant_destroy(&args[2]); args[2]=bytes_variant(p,n);
 }
 const godot_variant *ptrs[]={&args[0],&args[1],&args[2]};
 godot_string name; g->godot_string_new(&name); g->godot_string_parse_utf8(&name,"bridge_call");
 godot_variant_call_error error;
 godot_variant result=g->godot_variant_call(&r->host,&name,ptrs,3,&error);
 g->godot_string_destroy(&name); for(int i=0;i<3;i++)g->godot_variant_destroy(&args[i]);
 JSValue out=JS_NULL;
 if(error.error!=GODOT_CALL_ERROR_CALL_OK) out=JS_ThrowInternalError(ctx,"Godot bridge call failed: %d",error.error);
 else if(g->godot_variant_get_type(&result)==GODOT_VARIANT_TYPE_POOL_BYTE_ARRAY) {
  godot_pool_byte_array p=g->godot_variant_as_pool_byte_array(&result);
  godot_pool_byte_array_read_access *a=g->godot_pool_byte_array_read(&p);
  out=JS_NewArrayBufferCopy(ctx,g->godot_pool_byte_array_read_access_ptr(a),g->godot_pool_byte_array_size(&p));
  g->godot_pool_byte_array_read_access_destroy(a); g->godot_pool_byte_array_destroy(&p);
 } else if(g->godot_variant_get_type(&result)==GODOT_VARIANT_TYPE_STRING) {
  godot_string s=g->godot_variant_as_string(&result); godot_char_string c=g->godot_string_utf8(&s);
  out=JS_NewString(ctx,g->godot_char_string_get_data(&c));
  g->godot_char_string_destroy(&c); g->godot_string_destroy(&s);
 }
 g->godot_variant_destroy(&result); return out;
}
static void *create(godot_object *obj,void *data) {
 (void)obj;(void)data; Runtime *r=calloc(1,sizeof(*r)); r->host=nil();
 r->rt=JS_NewRuntime(); JS_SetMemoryLimit(r->rt,TITANIC_HEAP_LIMIT); JS_SetMaxStackSize(r->rt,2*1024*1024);
 r->ctx=JS_NewContext(r->rt); JS_SetContextOpaque(r->ctx,r);
 JSValue glob=JS_GetGlobalObject(r->ctx); JS_SetPropertyStr(r->ctx,glob,"__native",JS_NewCFunction(r->ctx,native_call,"__native",3));
 JS_SetPropertyStr(r->ctx,glob,"__indexedRGBA",JS_NewCFunction(r->ctx,titanic_indexed_rgba,"__indexedRGBA",4));
 titanic_install_codecs(r->ctx,glob);
 JS_SetPropertyStr(r->ctx,glob,"__runtimeMemory",JS_NewCFunction(r->ctx,titanic_memory_stats,"__runtimeMemory",0));JS_FreeValue(r->ctx,glob);
 return r;
}
static void destroy(godot_object *obj,void *data,void *user) {
 (void)obj;(void)data; Runtime *r=user; JS_FreeContext(r->ctx); JS_FreeRuntime(r->rt);
 g->godot_variant_destroy(&r->host); free(r->error); free(r);
}
static JSValue evaluate(Runtime *r,godot_variant *arg) {
 free(r->error); r->error=NULL; titanic_prepare_memory(r->rt);
 godot_string s=g->godot_variant_as_string(arg); godot_char_string c=g->godot_string_utf8(&s);
 JSValue v=JS_Eval(r->ctx,g->godot_char_string_get_data(&c),g->godot_char_string_length(&c),"engine.js",JS_EVAL_TYPE_GLOBAL);
 g->godot_char_string_destroy(&c); g->godot_string_destroy(&s);
 if(JS_IsException(v)) remember_error(r); return v;
}
static void jobs(Runtime *r) {
 JSContext *ctx;
 for(int i=0;i<4096 && JS_IsJobPending(r->rt);i++) if(JS_ExecutePendingJob(r->rt,&ctx)<0) { remember_error(r); break; }
}
static godot_variant initialize(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m; Runtime *r=u; if(n!=2)return nil();
 g->godot_variant_destroy(&r->host); g->godot_variant_new_copy(&r->host,a[0]);
 JSValue v=evaluate(r,a[1]); JS_FreeValue(r->ctx,v); jobs(r); return string_variant(r->error?r->error:"");
}
static godot_variant execute(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m; Runtime *r=u; if(n!=1)return nil();
 JSValue v=evaluate(r,a[0]); JS_FreeValue(r->ctx,v); jobs(r); return string_variant(r->error?r->error:"");
}
static godot_variant query(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m; Runtime *r=u; if(n!=1)return nil(); JSValue v=evaluate(r,a[0]);
 godot_variant out=nil(); if(!JS_IsException(v)) {
  const char *s=JS_ToCString(r->ctx,v); if(s) {g->godot_variant_destroy(&out); out=string_variant(s);JS_FreeCString(r->ctx,s);}
 } JS_FreeValue(r->ctx,v); return out;
}
static godot_variant buffer(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m; Runtime *r=u; if(n!=1)return nil(); JSValue v=evaluate(r,a[0]);
 godot_variant out=bytes_variant(NULL,0);
 if(!JS_IsException(v)&&!JS_IsNull(v)&&!JS_IsUndefined(v)) {
  size_t len=0; uint8_t *p=JS_GetArrayBuffer(r->ctx,&len,v);
  if(p) {g->godot_variant_destroy(&out);out=bytes_variant(p,len);} else remember_error(r);
 } JS_FreeValue(r->ctx,v); return out;
}
void GDN_EXPORT godot_gdnative_init(godot_gdnative_init_options *options) {
 g=options->api_struct; for(unsigned i=0;i<g->num_extensions;i++) if(g->extensions[i]->type==GDNATIVE_EXT_NATIVESCRIPT) ns=(void*)g->extensions[i];
}
void GDN_EXPORT godot_gdnative_terminate(godot_gdnative_terminate_options *o) {(void)o;}
void GDN_EXPORT godot_nativescript_init(void *handle) {
 godot_instance_create_func c={create,NULL,NULL}; godot_instance_destroy_func d={destroy,NULL,NULL};
 ns->godot_nativescript_register_class(handle,"DreamRuntime","Reference",c,d);
 godot_method_attributes attrs={GODOT_METHOD_RPC_MODE_DISABLED};
 godot_instance_method methods[]={{initialize,NULL,NULL},{execute,NULL,NULL},{query,NULL,NULL},{buffer,NULL,NULL}};
 const char *names[]={"initialize","execute","query","buffer"};
 for(int i=0;i<4;i++) ns->godot_nativescript_register_method(handle,"DreamRuntime",names[i],attrs,methods[i]);
}
