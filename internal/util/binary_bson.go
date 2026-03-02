package util

func bsonDocLen(data []byte) int {
	return int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
}

func isValidBSONElementType(b byte) bool {
	return b == 0x00 || (b >= 0x01 && b <= 0x13) || b == 0x7f || b == 0xff
}

func detectBSON(data []byte) *BinaryFormat {
	if len(data) < 5 {
		return nil
	}

	docLen := bsonDocLen(data)
	if docLen < 5 || docLen > 16*1024*1024 {
		return nil
	}

	if len(data) >= docLen && data[docLen-1] != 0x00 {
		return nil
	}
	if docLen < len(data) {
		return nil
	}

	if len(data) > 4 && isValidBSONElementType(data[4]) {
		return &BinaryFormat{Name: "bson", Confidence: 0.65, Details: "document"}
	}
	if len(data) <= 4 {
		return &BinaryFormat{Name: "bson", Confidence: 0.5, Details: "document (partial)"}
	}

	return nil
}
