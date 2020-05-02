func() {
    defer func() {
		if err := recover(); err != nil {
		    // store err to other place
		}
	}
	// do something
}()