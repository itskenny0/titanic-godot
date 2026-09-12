#include "core/version.h"
#if VERSION_MAJOR >= 4
#include "modules/register_module_types.h"
void initialize_titanic_module(ModuleInitializationLevel level);
void uninitialize_titanic_module(ModuleInitializationLevel level);
#else
void register_titanic_types();
void unregister_titanic_types();
#endif
