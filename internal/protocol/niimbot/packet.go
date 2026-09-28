package niimbot

import "fmt"

const (
	ServiceUUID        = "e7810a71-73ae-499d-8c15-faa9aef0c3f2"
	CharacteristicUUID = "bef8d6c9-9c21-4c9e-b632-bd58c1009f9f"

	CmdConnect          = 0xC1
	CmdGetInfo          = 0x40
	CmdHeartbeat        = 0xDC
	CmdPrintStatus      = 0xA3
	CmdPrinterInfo      = 0xA5
	CmdSetDensity       = 0x21
	CmdSetLabelType     = 0x23
	CmdPrintStart       = 0x01
	CmdPageStart        = 0x03
	CmdSetPageSize      = 0x13
	CmdPrintQuantity    = 0x15
	CmdPrintClear       = 0x20
	CmdBitmapRowIndexed = 0x83
	CmdEmptyRow         = 0x84
	CmdBitmapRow        = 0x85
	CmdPageEnd          = 0xE3
	CmdPrintEnd         = 0xF3
)

type Packet struct {
	Command  byte
	Length   int
	Payload  []byte
	Checksum byte
}

func ConnectPacket() []byte {
	return framedPacket(CmdConnect, []byte{0x01}, true)
}

func GetInfoPacket(key byte) []byte {
	return framedPacket(CmdGetInfo, []byte{key}, false)
}

func HeartbeatPacket() []byte {
	return framedPacket(CmdHeartbeat, []byte{0x01}, false)
}

func PrintStatusPacket() []byte {
	return framedPacket(CmdPrintStatus, []byte{0x01}, false)
}

func PrinterInfoPacket() []byte {
	return framedPacket(CmdPrinterInfo, []byte{0x01}, false)
}

func SetDensityPacket(density byte) []byte {
	return framedPacket(CmdSetDensity, []byte{density}, false)
}

func SetLabelTypePacket(labelType byte) []byte {
	return framedPacket(CmdSetLabelType, []byte{labelType}, false)
}

func PrintStartPacket() []byte {
	return framedPacket(CmdPrintStart, []byte{0x01}, false)
}

func PrintStartV4Packet(totalPages uint16, color byte, speed byte) []byte {
	payload := []byte{byte(totalPages >> 8), byte(totalPages), 0x00, 0x00, 0x00, 0x00, color, speed, 0x00}
	return framedPacket(CmdPrintStart, payload, false)
}

func PrintStartPagesPacket(totalPages uint16, color byte) []byte {
	payload := []byte{byte(totalPages >> 8), byte(totalPages), 0x00, 0x00, 0x00, 0x00, color}
	return framedPacket(CmdPrintStart, payload, false)
}

func PrintClearPacket() []byte {
	return framedPacket(CmdPrintClear, []byte{0x01}, false)
}

func PageStartPacket() []byte {
	return framedPacket(CmdPageStart, []byte{0x01}, false)
}

func SetPageSizePacket(rows, cols int) []byte {
	return framedPacket(CmdSetPageSize, []byte{byte(rows >> 8), byte(rows), byte(cols >> 8), byte(cols)}, false)
}

func SetPageSizeV4Packet(rows, cols, quantity int) []byte {
	payload := []byte{
		byte(rows >> 8), byte(rows),
		byte(cols >> 8), byte(cols),
		byte(quantity >> 8), byte(quantity),
		0x00, 0x00,
		0x00, 0x00, 0x00,
		0x00, 0x00,
	}
	return framedPacket(CmdSetPageSize, payload, false)
}

func SetPageSizePagesPacket(rows, cols, quantity int) []byte {
	payload := []byte{byte(rows >> 8), byte(rows), byte(cols >> 8), byte(cols), byte(quantity >> 8), byte(quantity)}
	return framedPacket(CmdSetPageSize, payload, false)
}

func PrintQuantityPacket(qty int) []byte {
	return framedPacket(CmdPrintQuantity, []byte{byte(qty >> 8), byte(qty)}, false)
}

func EmptyRowPacket(pos, repeat int) []byte {
	return framedPacket(CmdEmptyRow, []byte{byte(pos >> 8), byte(pos), byte(repeat)}, false)
}

func BitmapRowPacket(pos int, rowBytes []byte) []byte {
	return BitmapRowPacketRepeated(pos, 1, rowBytes)
}

func BitmapRowPacketRepeated(pos, repeat int, rowBytes []byte) []byte {
	counts := splitBlackCounts(rowBytes)
	payload := make([]byte, 0, 6+len(rowBytes))
	payload = append(payload, byte(pos>>8), byte(pos))
	payload = append(payload, counts[0], counts[1], counts[2], byte(repeat))
	payload = append(payload, rowBytes...)
	return framedPacket(CmdBitmapRow, payload, false)
}

func BitmapRowIndexedPacket(pos int, rowBytes []byte) []byte {
	return BitmapRowIndexedPacketRepeated(pos, 1, rowBytes)
}

func BitmapRowIndexedPacketRepeated(pos, repeat int, rowBytes []byte) []byte {
	counts := splitBlackCounts(rowBytes)
	indexes := indexPixels(rowBytes)
	payload := make([]byte, 0, 6+len(indexes))
	payload = append(payload, byte(pos>>8), byte(pos))
	payload = append(payload, counts[0], counts[1], counts[2], byte(repeat))
	payload = append(payload, indexes...)
	return framedPacket(CmdBitmapRowIndexed, payload, false)
}

func PageEndPacket() []byte {
	return framedPacket(CmdPageEnd, []byte{0x01}, false)
}

func PrintEndPacket() []byte {
	return framedPacket(CmdPrintEnd, []byte{0x01}, false)
}

func framedPacket(cmd byte, data []byte, connectPrefix bool) []byte {
	packet := make([]byte, 0, len(data)+8)
	if connectPrefix {
		packet = append(packet, 0x03)
	}
	packet = append(packet, 0x55, 0x55, cmd, byte(len(data)))
	packet = append(packet, data...)
	packet = append(packet, xorChecksum(cmd, data), 0xAA, 0xAA)
	return packet
}

func xorChecksum(cmd byte, data []byte) byte {
	checksum := cmd ^ byte(len(data))
	for _, b := range data {
		checksum ^= b
	}
	return checksum
}

func splitBlackCounts(rowBytes []byte) [3]byte {
	var counts [3]byte
	chunk := len(rowBytes) / 3
	if chunk == 0 {
		chunk = len(rowBytes)
	}
	for idx, b := range rowBytes {
		bucket := 2
		if chunk > 0 {
			bucket = idx / chunk
			if bucket > 2 {
				bucket = 2
			}
		}
		counts[bucket] += byte(popcount8(b))
	}
	return counts
}

func countBlackPixels(rowBytes []byte) int {
	total := 0
	for _, b := range rowBytes {
		total += popcount8(b)
	}
	return total
}

func CountBlackPixelsForDebug(rowBytes []byte) int {
	return countBlackPixels(rowBytes)
}

func indexPixels(rowBytes []byte) []byte {
	indexes := make([]byte, 0, countBlackPixels(rowBytes)*2)
	for bytePos, value := range rowBytes {
		for bitPos := 0; bitPos < 8; bitPos++ {
			if value&(1<<uint(7-bitPos)) == 0 {
				continue
			}
			idx := bytePos*8 + bitPos
			indexes = append(indexes, byte(idx>>8), byte(idx))
		}
	}
	return indexes
}

func popcount8(v byte) int {
	count := 0
	for v != 0 {
		count += int(v & 1)
		v >>= 1
	}
	return count
}

func ParseFramedPacket(buf []byte) (Packet, error) {
	start := 0
	if len(buf) > 0 && buf[0] == 0x03 {
		start = 1
	}
	if len(buf[start:]) < 7 {
		return Packet{}, fmt.Errorf("packet too short")
	}
	if buf[start] != 0x55 || buf[start+1] != 0x55 {
		return Packet{}, fmt.Errorf("invalid header")
	}
	cmd := buf[start+2]
	length := int(buf[start+3])
	need := start + 4 + length + 3
	if len(buf) < need {
		return Packet{}, fmt.Errorf("truncated packet")
	}
	payload := append([]byte(nil), buf[start+4:start+4+length]...)
	checksum := buf[start+4+length]
	if buf[start+5+length] != 0xAA || buf[start+6+length] != 0xAA {
		return Packet{}, fmt.Errorf("invalid footer")
	}
	if xorChecksum(cmd, payload) != checksum {
		return Packet{}, fmt.Errorf("checksum mismatch")
	}
	return Packet{Command: cmd, Length: length, Payload: payload, Checksum: checksum}, nil
}

func AckFor(cmd byte) byte {
	return cmd + 1
}
