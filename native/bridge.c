/* GPL-3.0-or-later. Godot 3 GDNative adapter for the Go gameplay runtime. */
#include <gdnative_api_struct.gen.h>
#include "module/player_api.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

static const godot_gdnative_core_api_struct *g;
static const godot_gdnative_ext_nativescript_api_struct *ns;
typedef struct { uintptr_t handle; godot_variant host; } Runtime;
static godot_variant nil(void) { godot_variant v; g->godot_variant_new_nil(&v); return v; }
static godot_variant string_bytes(const char *s,int n) {
 godot_string gs; g->godot_string_new(&gs); g->godot_string_parse_utf8_with_len(&gs,s?s:"",n);
 godot_variant v; g->godot_variant_new_string(&v,&gs); g->godot_string_destroy(&gs); return v;
}
static godot_variant string_variant(const char *s) { return string_bytes(s,(int)strlen(s)); }
static godot_variant bytes_variant(const uint8_t *data,int64_t len) {
 godot_pool_byte_array p; g->godot_pool_byte_array_new(&p); g->godot_pool_byte_array_resize(&p,(int)len);
 if(len) { godot_pool_byte_array_write_access *w=g->godot_pool_byte_array_write(&p);
 memcpy(g->godot_pool_byte_array_write_access_ptr(w),data,(size_t)len); g->godot_pool_byte_array_write_access_destroy(w); }
 godot_variant v; g->godot_variant_new_pool_byte_array(&v,&p); g->godot_pool_byte_array_destroy(&p); return v;
}
static void copy_result(TaootResult *out,const void *data,int64_t size,int kind) {
 out->data=size?malloc((size_t)size):NULL;out->size=size;out->kind=kind;
 if(size)memcpy(out->data,data,(size_t)size);
}
static void platform_call(uintptr_t owner,const char *method,const char *json,const uint8_t *data,int64_t size,TaootResult *out) {
 Runtime *r=(Runtime*)owner;
 godot_variant args[3]={string_variant(method),string_variant(json),nil()};
 if(data){g->godot_variant_destroy(&args[2]);args[2]=bytes_variant(data,size);}
 const godot_variant *ptrs[]={&args[0],&args[1],&args[2]};
 godot_string name;g->godot_string_new(&name);g->godot_string_parse_utf8(&name,"bridge_call");
 godot_variant_call_error error;godot_variant result=g->godot_variant_call(&r->host,&name,ptrs,3,&error);
 g->godot_string_destroy(&name);for(int i=0;i<3;i++)g->godot_variant_destroy(&args[i]);
 if(error.error!=GODOT_CALL_ERROR_CALL_OK){const char *text="Godot platform callback failed";copy_result(out,text,strlen(text),-1);}
 else if(g->godot_variant_get_type(&result)==GODOT_VARIANT_TYPE_POOL_BYTE_ARRAY){
  godot_pool_byte_array p=g->godot_variant_as_pool_byte_array(&result);godot_pool_byte_array_read_access *rd=g->godot_pool_byte_array_read(&p);
  copy_result(out,g->godot_pool_byte_array_read_access_ptr(rd),g->godot_pool_byte_array_size(&p),2);
  g->godot_pool_byte_array_read_access_destroy(rd);g->godot_pool_byte_array_destroy(&p);
 }else if(g->godot_variant_get_type(&result)==GODOT_VARIANT_TYPE_STRING){
  godot_string s=g->godot_variant_as_string(&result);godot_char_string c=g->godot_string_utf8(&s);
  copy_result(out,g->godot_char_string_get_data(&c),g->godot_char_string_length(&c),1);
  g->godot_char_string_destroy(&c);g->godot_string_destroy(&s);
 }
 g->godot_variant_destroy(&result);
}
static void *create(godot_object *o,void *data) {(void)o;(void)data;Runtime *r=calloc(1,sizeof(*r));r->host=nil();return r;}
static void destroy(godot_object *o,void *data,void *user) {
 (void)o;(void)data;Runtime *r=user;if(r->handle)taoot_player_close(r->handle);
 g->godot_variant_destroy(&r->host);free(r);
}
static godot_variant initialize(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m;Runtime *r=u;if(n!=1)return string_variant("initialize needs its owning node");
 if(r->handle)taoot_player_close(r->handle);
 g->godot_variant_destroy(&r->host);g->godot_variant_new_copy(&r->host,a[0]);
 r->handle=taoot_player_new((uintptr_t)r,(void*)platform_call);
 return string_variant(r->handle?"":"Could not initialize Go runtime");
}
static TaootResult call(Runtime *r,int n,godot_variant **a) {
 TaootResult out={0};if(n<1||n>2||!r->handle){const char *s="Invalid runtime call";copy_result(&out,s,strlen(s),-1);return out;}
 godot_string method=g->godot_variant_as_string(a[0]);godot_char_string mc=g->godot_string_utf8(&method);
 godot_variant fallback=string_variant("{}");godot_string args=g->godot_variant_as_string(n>1?a[1]:&fallback);godot_char_string ac=g->godot_string_utf8(&args);
 taoot_player_call(r->handle,(char*)g->godot_char_string_get_data(&mc),(char*)g->godot_char_string_get_data(&ac),&out);
 g->godot_char_string_destroy(&mc);g->godot_string_destroy(&method);g->godot_char_string_destroy(&ac);g->godot_string_destroy(&args);g->godot_variant_destroy(&fallback);return out;
}
static godot_variant execute(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m;TaootResult out=call(u,n,a);godot_variant result=out.kind<0?string_bytes((char*)out.data,(int)out.size):string_variant("");free(out.data);return result;
}
static godot_variant query(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m;TaootResult out=call(u,n,a);godot_variant result=out.kind==1?string_bytes((char*)out.data,(int)out.size):string_variant("null");free(out.data);return result;
}
static godot_variant buffer(godot_object *o,void *m,void *u,int n,godot_variant **a) {
 (void)o;(void)m;TaootResult out=call(u,n,a);godot_variant result=out.kind==2?bytes_variant(out.data,out.size):bytes_variant(NULL,0);free(out.data);return result;
}
void GDN_EXPORT godot_gdnative_init(godot_gdnative_init_options *options) {
 g=options->api_struct;for(unsigned i=0;i<g->num_extensions;i++)if(g->extensions[i]->type==GDNATIVE_EXT_NATIVESCRIPT)ns=(void*)g->extensions[i];
}
void GDN_EXPORT godot_gdnative_terminate(godot_gdnative_terminate_options *o){(void)o;}
void GDN_EXPORT godot_nativescript_init(void *handle) {
 godot_instance_create_func c={create,NULL,NULL};godot_instance_destroy_func d={destroy,NULL,NULL};
 ns->godot_nativescript_register_class(handle,"DreamRuntime","Reference",c,d);
 godot_method_attributes attrs={GODOT_METHOD_RPC_MODE_DISABLED};
 godot_instance_method methods[]={{initialize,NULL,NULL},{execute,NULL,NULL},{query,NULL,NULL},{buffer,NULL,NULL}};
 const char *names[]={"initialize","execute","query","buffer"};
 for(int i=0;i<4;i++)ns->godot_nativescript_register_method(handle,"DreamRuntime",names[i],attrs,methods[i]);
}
