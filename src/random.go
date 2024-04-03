package main

var seed = uint32(1)

func CloudRandomU16() uint16 {
	seed = uint32(uint64(seed)*1103515245 + 12345)
	return uint16(seed >> 15)
}

func CloudRandom(max int) int {
	// FIXME: Discard and re-flip for better uniformity
	return int(CloudRandomU16()) % max
}
