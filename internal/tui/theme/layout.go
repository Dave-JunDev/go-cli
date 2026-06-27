package theme

func ContentWidth(totalWidth int) int {
	if totalWidth < 40 {
		return 40
	}
	return totalWidth
}

func ViewportWidth(totalWidth int) int {
	w := totalWidth - 4
	if w < 40 {
		w = 40
	}
	return w
}

func ViewportHeight(totalHeight, reserved int) int {
	h := totalHeight - reserved
	if h < 5 {
		h = 5
	}
	return h
}

func StatusBarWidth(totalWidth int) int {
	if totalWidth < 80 {
		return 80
	}
	return totalWidth
}
