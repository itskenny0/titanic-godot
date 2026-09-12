/* GPL-3.0-or-later */
#ifndef TITANIC_RUNTIME_H
#define TITANIC_RUNTIME_H
#include "core/version.h"
#if VERSION_MAJOR >= 4
#include "core/object/ref_counted.h"
using RuntimeBase = RefCounted;
using Bytes = PackedByteArray;
#else
#include "core/reference.h"
using RuntimeBase = Reference;
using Bytes = PoolByteArray;
#endif
extern "C" {
#include "quickjs.h"
}
class DreamRuntime : public RuntimeBase {
 GDCLASS(DreamRuntime, RuntimeBase);
 JSRuntime *rt = nullptr;
 JSContext *ctx = nullptr;
 Object *host = nullptr;
 String last_error;
 JSValue eval(const String &code);
 void pump();
 void exception();
 static JSValue native_call(JSContext *, JSValueConst, int, JSValueConst *);
protected:
 static void _bind_methods();
public:
 DreamRuntime();
 ~DreamRuntime();
 String initialize(Object *owner, const String &code);
 String execute(const String &code);
 String query(const String &code);
 Bytes buffer(const String &code);
};
#endif
