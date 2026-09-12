package label

type ElementType string

const (
	ElementText ElementType = "text"
	ElementQR   ElementType = "qr"
)

type Element struct {
	ID string

	Type ElementType

	XMM      float64
	YMM      float64
	WidthMM  float64
	HeightMM float64

	Text *TextElement
	QR   *QRElement
}

type TextElement struct {
	Value    string
	FontSize float64
	FontPath string
	Bold     bool
	Align    string
}

type QRElement struct {
	Value string
}
