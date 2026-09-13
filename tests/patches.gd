extends SceneTree

class PatchStub:
	extends Reference
	func execute(_method, _args = "{}"):
		return ""
	func query(_method):
		return JSON.print({"state": "error", "error": "Test transfer cancelled", "bytes": 0, "total": 0})

var failed = false
func check(ok, message):
	if not ok:
		failed = true
		printerr("FAIL: ", message)
func _init(): call_deferred("begin")
func begin():
	var player = load("res://Main.tscn").instance()
	get_root().add_child(player)
	player.set_process(false)
	player.close_modal()
	# Exercise the missing-patch surface even on a developer's bundled checkout.
	player.patch_manifest.files = {"missing-test-patch.SET": {}}
	player.show_patch_picker(true)
	yield(self, "idle_frame")
	yield(self, "idle_frame")
	var bounds = player.modal.get_global_rect()
	check(bounds.position.x >= 0 and bounds.position.y >= 0 and bounds.end.x <= player.layout_size.x and bounds.end.y <= player.layout_size.y, "download chooser fits handheld viewport")
	check(player.patch_buttons.size() == 2, "download and file-picker sources available")
	for check_box in player.patch_boxes.values():
		check(check_box.has_icon_override("checked") and check_box.has_icon_override("unchecked"), "patch states have explicit icons")
		check(check_box.get_icon("checked").get_width() == 24, "check mark is large enough on handhelds")
	player.patch_select_all(true)
	yield(self, "idle_frame")
	VisualServer.force_draw()
	var screenshot = get_root().get_texture().get_data()
	screenshot.flip_y()
	screenshot.save_png("user://patch-checkboxes.png")
	player.patch_runtime = PatchStub.new()
	player.patch_start("test.zip")
	check(player.patch_busy, "transfer started")
	for source in player.patch_buttons:
		check(source.disabled, "duplicate transfer disabled")
	player.patch_apply()
	check(player.modal != null and player.patch_busy, "cannot apply incomplete patches")
	player.poll_patches(0.25)
	check(not player.patch_busy, "failed transfer can be retried")
	check(player.patch_status_label.text == "Test transfer cancelled", "transfer error shown")
	for source in player.patch_buttons:
		check(not source.disabled, "transfer source restored")
	player.patch_runtime = PatchStub.new()
	player.patch_start("test.zip")
	player.close_modal()
	yield(self, "idle_frame")
	player.poll_patches(0.25)
	check(not player.patch_busy, "transfer completion survives closing the chooser")
	print("PATCH UI ", "FAIL" if failed else "PASS")
	quit(1 if failed else 0)
