extends Reference
# Display changes only. Every answer still maps to its authored 512x384 target.
var size = Vector2(683,384)
var choices = []
var subtitle = Rect2()

func configure(viewport_size, count, touch = false):
	size = viewport_size
	choices.clear()
	var width = min(512, size.x - (280 if touch else 48))
	var top = size.y - 12 - count * 27
	for i in range(count):
		choices.append(Rect2((size.x-width)/2,top+i*27,width,24))
	subtitle = Rect2((size.x-512)/2,max(8,top-48),512,40)

func screen_to_game(point):
	for i in range(choices.size()):
		var r = choices[i]
		if r.has_point(point):
			return Vector2((point.x-r.position.x)*512/r.size.x,264+i*24+(point.y-r.position.y))
	# Speech and empty space never masquerade as answer targets.
	return Vector2(point.x*512/size.x, min(223,point.y*264/size.y)) if Rect2(Vector2.ZERO,size).has_point(point) else Vector2(-1,-1)

func game_to_screen(point):
	var index = int((point.y-264)/24)
	if point.y >= 264 and index >= 0 and index < choices.size():
		return choices[index].position+Vector2(point.x*choices[index].size.x/512,12)
	return point*size/Vector2(512,264)
