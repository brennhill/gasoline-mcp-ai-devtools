package util

import "fmt"

func isValidProtobufWireType(wireType byte) bool {
	return wireType == 0 || wireType == 1 || wireType == 2 || wireType == 5
}

func protobufFieldDetail(fieldNumber byte, wireTypeStr string) string {
	return fmt.Sprintf("field %d, %s", fieldNumber, wireTypeStr)
}

func isValidVarint(data []byte) bool {
	for i := 1; i < len(data) && i < 10; i++ {
		if data[i]&0x80 == 0 {
			return true
		}
	}
	return len(data) < 10
}

func detectProtobufLengthDelimited(data []byte, fieldNumber byte) *BinaryFormat {
	if data[1]&0x80 != 0 {
		return &BinaryFormat{Name: "protobuf", Confidence: 0.6, Details: protobufFieldDetail(fieldNumber, "length-delimited")}
	}

	length := int(data[1])
	if length > 0 && len(data) >= 2+length {
		return &BinaryFormat{Name: "protobuf", Confidence: 0.7, Details: protobufFieldDetail(fieldNumber, "length-delimited")}
	}
	return nil
}

func detectProtobuf(data []byte) *BinaryFormat {
	if len(data) < 2 {
		return nil
	}

	wireType := data[0] & 0x07
	fieldNumber := data[0] >> 3

	if !isValidProtobufWireType(wireType) || fieldNumber == 0 || fieldNumber > 15 {
		return nil
	}

	switch wireType {
	case 0:
		if isValidVarint(data) {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.7, Details: protobufFieldDetail(fieldNumber, "varint")}
		}
	case 1:
		if len(data) >= 9 {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.65, Details: protobufFieldDetail(fieldNumber, "fixed64")}
		}
	case 2:
		return detectProtobufLengthDelimited(data, fieldNumber)
	case 5:
		if len(data) >= 5 {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.65, Details: protobufFieldDetail(fieldNumber, "fixed32")}
		}
	}

	return nil
}
