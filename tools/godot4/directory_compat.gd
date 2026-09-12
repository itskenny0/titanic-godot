extends RefCounted
var directory: DirAccess
func open(path):
	directory = DirAccess.open(path)
	return DirAccess.get_open_error()
func list_dir_begin(skip_navigational = true, skip_hidden = true):
	directory.include_hidden = not skip_hidden
	directory.include_navigational = not skip_navigational
	return directory.list_dir_begin()
func list_dir_end(): directory.list_dir_end()
func get_next(): return directory.get_next()
func current_is_dir(): return directory.current_is_dir()
func make_dir_recursive(path): return DirAccess.make_dir_recursive_absolute(path)
func copy(source, target): return DirAccess.copy_absolute(source, target)
func rename(source, target): return DirAccess.rename_absolute(source, target)
func remove(path): return DirAccess.remove_absolute(path)
