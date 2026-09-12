extends RefCounted
const READ = FileAccess.READ
const WRITE = FileAccess.WRITE
var file: FileAccess
func open(path, mode):
	file = FileAccess.open(path, mode)
	return FileAccess.get_open_error()
func close():
	if file: file.close()
func get_as_text(): return file.get_as_text()
func get_length(): return file.get_length()
func get_buffer(length): return file.get_buffer(length)
func get_error(): return file.get_error()
func store_buffer(bytes): file.store_buffer(bytes)
func flush(): file.flush()
func seek(position): file.seek(position)
func get_32(): return file.get_32()
func file_exists(path): return FileAccess.file_exists(path)
func get_sha256(path): return FileAccess.get_sha256(path)
func get_modified_time(path): return FileAccess.get_modified_time(path)
