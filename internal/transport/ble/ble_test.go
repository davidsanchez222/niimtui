package ble

import (
	"testing"

	"niimtui/internal/protocol/niimbot"
)

func TestFindResponseSinceAfterNotificationHistoryWraps(t *testing.T) {
	c := &connection{meta: make(map[string]any)}
	oldResponse := []byte{0x55, 0x55, 0x31, 0x01, 0x01, 0x31, 0xaa, 0xaa}
	newResponse := []byte{0x55, 0x55, 0x31, 0x01, 0x02, 0x32, 0xaa, 0xaa}
	for range 50 {
		c.recordNotification(oldResponse)
	}
	before := c.notificationCount()
	if _, found := c.findResponseSince(before, 0x31); found {
		t.Fatal("matched a response received before the request")
	}
	c.recordNotification(newResponse)
	got, found := c.findResponseSince(before, 0x31)
	if !found || len(got) != 1 || got[0] != 0x02 {
		t.Fatalf("findResponseSince() = %v, %v; want new response 02", got, found)
	}
}

func TestSendRowsCoalescesRepeatedRowsWithoutOverflow(t *testing.T) {
	rows := make([][]byte, 0, 522)
	for range 260 {
		rows = append(rows, []byte{0})
	}
	for range 260 {
		rows = append(rows, []byte{0x80})
	}
	rows = append(rows, []byte{0xff}, []byte{0})
	type sentPacket struct {
		name string
		data []byte
	}
	var packets []sentPacket
	err := sendRows(rows, func(name string, data []byte) error {
		packets = append(packets, sentPacket{name, data})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name   string
		cmd    byte
		pos    int
		repeat byte
	}{
		{"empty_row", niimbot.CmdEmptyRow, 0, 255},
		{"empty_row", niimbot.CmdEmptyRow, 255, 5},
		{"bitmap_row_indexed", niimbot.CmdBitmapRowIndexed, 260, 255},
		{"bitmap_row_indexed", niimbot.CmdBitmapRowIndexed, 515, 5},
		{"bitmap_row", niimbot.CmdBitmapRow, 520, 1},
		{"empty_row", niimbot.CmdEmptyRow, 521, 1},
	}
	if len(packets) != len(want) {
		t.Fatalf("sent %d packets for %d rows, want %d", len(packets), len(rows), len(want))
	}
	for i, p := range packets {
		position := int(p.data[4])<<8 | int(p.data[5])
		repeatOffset := 9
		if p.name == "empty_row" {
			repeatOffset = 6
		}
		if p.name != want[i].name || p.data[2] != want[i].cmd || position != want[i].pos || p.data[repeatOffset] != want[i].repeat {
			t.Errorf("packet %d: %q cmd %02x pos %d repeat %d; want %+v", i, p.name, p.data[2], position, p.data[repeatOffset], want[i])
		}
	}
}
