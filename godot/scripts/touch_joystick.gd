extends Node2D
signal door_tapped
signal direction_pressed(direction)

var radius = 58.0
var finger = -1
var offset = Vector2.ZERO
var direction = ""
var touch_origin = Vector2.ZERO
var touch_started = 0
var dragged = false
var started_in_center = false
const DIRECTIONS = ["up", "down", "left", "right"]
const ENGAGE_ZONE = 0.42
const RELEASE_ZONE = 0.28
const AXIS_SWITCH_RATIO = 1.5

func _ready():
	for name in DIRECTIONS:
		if not InputMap.has_action("touch_" + name):
			InputMap.add_action("touch_" + name)

func configure(center, size):
	reset()
	position = center
	radius = size
	update()

func reset():
	finger = -1
	dragged = false
	offset = Vector2.ZERO
	set_direction("")
	update()

func set_direction(next):
	if direction == next:
		return
	if not direction.empty():
		Input.action_release("touch_" + direction)
	direction = next
	if not direction.empty():
		Input.action_press("touch_" + direction)
		emit_signal("direction_pressed", direction)

func move_stick(point):
	offset = point.normalized() * min(point.length(), radius * 0.65)
	var next = ""
	if point.length() >= radius * (ENGAGE_ZONE if direction.empty() else RELEASE_ZONE):
		if abs(point.x) > abs(point.y):
			next = "right" if point.x > 0 else "left"
		else:
			next = "down" if point.y > 0 else "up"
		# Keep the chosen axis through small diagonal thumb movements.
		if direction in ["left", "right"] and point.x * (-1 if direction == "left" else 1) > 0:
			if abs(point.y) < abs(point.x) * AXIS_SWITCH_RATIO:
				next = direction
		elif direction in ["up", "down"] and point.y * (-1 if direction == "up" else 1) > 0:
			if abs(point.x) < abs(point.y) * AXIS_SWITCH_RATIO:
				next = direction
	set_direction(next)
	update()

func _input(event):
	if not is_visible_in_tree():
		return
	if event is InputEventScreenTouch:
		if event.pressed and finger == -1 and to_local(event.position).length() <= radius:
			finger = event.index
			touch_origin = to_local(event.position)
			touch_started = OS.get_ticks_msec()
			dragged = false
			started_in_center = touch_origin.length() <= radius * 0.34
			move_stick(Vector2.ZERO if started_in_center else touch_origin)
			get_tree().set_input_as_handled()
		elif not event.pressed and event.index == finger:
			var tapped = started_in_center and not dragged and OS.get_ticks_msec() - touch_started <= 350 and to_local(event.position).length() <= radius * 0.34
			reset()
			if tapped:
				emit_signal("door_tapped")
			get_tree().set_input_as_handled()
	elif event is InputEventScreenDrag and event.index == finger:
		var movement = to_local(event.position) - touch_origin
		if movement.length() >= radius * 0.18:
			dragged = true
		if dragged:
			move_stick(movement if started_in_center else to_local(event.position))
		get_tree().set_input_as_handled()

func _notification(what):
	if what == NOTIFICATION_VISIBILITY_CHANGED and not is_visible_in_tree():
		reset()
	elif what == MainLoop.NOTIFICATION_WM_FOCUS_OUT:
		reset()

func _draw():
	draw_circle(Vector2.ZERO, radius, Color("263a4d"))
	draw_arc(Vector2.ZERO, radius - 1, 0, TAU, 64, Color("70889d"), 2, true)
	for axis in [Vector2.UP, Vector2.DOWN, Vector2.LEFT, Vector2.RIGHT]:
		draw_circle(axis * radius * 0.80, max(2, radius * 0.045), Color("aec4d5"))
	draw_circle(offset, radius * 0.34, Color("91adc2") if direction.empty() else Color("d3e3ef"))
