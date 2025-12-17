package mycache

type ByteView struct {
	bytes []byte
}

func (byteview ByteView) Len() int {
	return len(byteview.bytes)
}
