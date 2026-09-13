/* GPL-3.0-or-later */
#ifndef TITANIC_RUNTIME_H
#define TITANIC_RUNTIME_H
#include "core/version.h"
#include "player_api.h"
#if VERSION_MAJOR >= 4
#include "core/object/ref_counted.h"
using RuntimeBase = RefCounted;
using Bytes = PackedByteArray;
#else
#include "core/reference.h"
using RuntimeBase = Reference;
using Bytes = PoolByteArray;
#endif
class DreamRuntime : public RuntimeBase {
 GDCLASS(DreamRuntime, RuntimeBase);
 uintptr_t handle = 0;
 Object *host = nullptr;
 TaootResult call(const String &method,const String &args);
 static void platform_call(uintptr_t,const char *,const char *,const uint8_t *,int64_t,TaootResult *);
protected:
 static void _bind_methods();
public:
 ~DreamRuntime();
 String initialize(Object *owner);
 String execute(const String &method,const String &args = "{}");
 String query(const String &method,const String &args = "{}");
 Bytes buffer(const String &method,const String &args = "{}");
};
#endif
