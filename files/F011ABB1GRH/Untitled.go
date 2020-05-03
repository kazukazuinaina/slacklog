	var attrs uint32 = FILE_ATTRIBUTE_NORMAL
	if perm&S_IWRITE == 0 {
		attrs = FILE_ATTRIBUTE_READONLY
	}
	h, e := CreateFile(pathp, access, sharemode, sa, createmode, attrs, 0)
