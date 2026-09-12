#include "register_types.h"
#include "runtime.h"
#if VERSION_MAJOR >= 4
void initialize_titanic_module(ModuleInitializationLevel level) { if(level == MODULE_INITIALIZATION_LEVEL_SCENE) ClassDB::register_class<DreamRuntime>(); }
void uninitialize_titanic_module(ModuleInitializationLevel level) {}
#else
void register_titanic_types() { ClassDB::register_class<DreamRuntime>(); }
void unregister_titanic_types() {}
#endif
