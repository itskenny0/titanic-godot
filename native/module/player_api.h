/* GPL-3.0-or-later. Result buffers have an explicit size, no trailing NUL.
 * Every returned buffer belongs to the caller and is released with free(). */
#ifndef TITANIC_PLAYER_API_H
#define TITANIC_PLAYER_API_H
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
typedef struct {
 uint8_t *data;
 int64_t size;
 int32_t kind; /* 0 absent, 1 UTF-8 JSON, 2 bytes, -1 error text */
} TaootResult;
typedef void (*TaootBridgeFn)(uintptr_t, const char *, const char *, const uint8_t *, int64_t, TaootResult *);
uintptr_t taoot_player_new(uintptr_t host, void *callback);
void taoot_player_close(uintptr_t handle);
void taoot_player_call(uintptr_t handle, char *method, char *args, TaootResult *out);
#ifdef __cplusplus
}
#endif
#endif
