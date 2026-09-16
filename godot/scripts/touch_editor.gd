extends Node2D
signal finished
signal reset_requested
var player
var dragged_control = ""
var drag_offset = Vector2.ZERO
var finger = -1
var bar
var layout_key = ""

func _ready():
	z_index = 110
	bar = HBoxContainer.new()
	bar.add_constant_override("separation",8)
	add_child(bar)
	for caption in ["Reset positions", "Done / lock"]:
		var b = Button.new()
		b.text = caption
		b.rect_min_size = Vector2(144,36)
		b.add_font_override("font",player.get_font_for("14px Arial"))
		bar.add_child(b)
		b.connect("pressed", self, "button_pressed", [caption])
	set_process(true)

func button_pressed(caption):
	if caption == "Reset positions":
		emit_signal("reset_requested")
	else:
		emit_signal("finished")

func _process(_delta):
	bar.rect_position = Vector2((player.layout_size.x-296)/2,8)
	if layout_key != player.touch_layout_key():
		layout_key = player.touch_layout_key()
		dragged_control = ""
		finger = -1
	update()

func _input(event):
	if event is InputEventKey and event.pressed and event.scancode == KEY_ESCAPE:
		emit_signal("finished")
		get_tree().set_input_as_handled()
		return
	var pressed = false
	var released = false
	var point = Vector2.ZERO
	var contact = -2
	if event is InputEventScreenTouch:
		point = event.position
		contact = event.index
		pressed = event.pressed
		released = not pressed
	elif event is InputEventScreenDrag:
		point = event.position
		contact = event.index
	elif event is InputEventMouseButton and event.button_index == BUTTON_LEFT:
		point = event.position
		pressed = event.pressed
		released = not pressed
	elif event is InputEventMouseMotion:
		point = event.position
	else:
		return
	if dragged_control.empty() and bar.get_global_rect().has_point(point):
		return
	if pressed and dragged_control.empty():
		for control in ["menu","space","escape","keyboard","joystick"]:
			var bounds = player.touch_control_bounds(control)
			if bounds.has_point(point):
				dragged_control = control
				finger = contact
				drag_offset = bounds.position + bounds.size/2 - point
				break
	if not dragged_control.empty() and contact == finger:
		player.move_touch_control(dragged_control,point+drag_offset)
		if released:
			dragged_control = ""
			finger = -1
	get_tree().set_input_as_handled()

func _draw():
	for control in ["menu","space","escape","keyboard","joystick"]:
		draw_rect(player.touch_control_bounds(control).grow(3),Color(0.85,0.77,0.56,0.8),false,1)
