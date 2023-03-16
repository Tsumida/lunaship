package utils

//go:inline
func Go(fn func()) {
	GoWithAction(fn, nil)
}

func GoWithAction(fn func(), panicAction func(r interface{})) {
	defer func() {
		if r := recover(); r != nil {
			if panicAction != nil {
				panicAction(r)
			}
		}
	}()

	fn()
}
