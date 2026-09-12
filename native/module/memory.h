/* GPL-3.0-or-later. Leave headroom for decoded images and room changes. */
#ifndef TITANIC_MEMORY_H
#define TITANIC_MEMORY_H
#include "quickjs.h"
#define TITANIC_HEAP_LIMIT ((size_t)384 * 1024 * 1024)
#define TITANIC_GC_CEILING ((size_t)256 * 1024 * 1024)
static void titanic_prepare_memory(JSRuntime *rt) {
 /* QuickJS raises its next collection threshold to 1.5 times the live heap.
  * That can exceed our hard limit, leaving unreachable room/viewer cycles
  * stranded until every subsequent image allocation fails. */
 if (JS_GetGCThreshold(rt) > TITANIC_GC_CEILING)
  JS_SetGCThreshold(rt, TITANIC_GC_CEILING);
}
static JSValue titanic_memory_stats(JSContext *ctx, JSValueConst self, int argc, JSValueConst *argv) {
 (void)self; (void)argc; (void)argv;
 JSMemoryUsage usage; JS_ComputeMemoryUsage(JS_GetRuntime(ctx), &usage);
 JSValue out=JS_NewObject(ctx);
 JS_SetPropertyStr(ctx,out,"allocated",JS_NewInt64(ctx,usage.malloc_size));
 JS_SetPropertyStr(ctx,out,"used",JS_NewInt64(ctx,usage.memory_used_size));
 JS_SetPropertyStr(ctx,out,"objects",JS_NewInt64(ctx,usage.obj_count));
 return out;
}
#endif
