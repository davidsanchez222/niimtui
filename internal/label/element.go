package label

type ElementType string

const (
	ElementText ElementType = "text"
)

type Element struct {
	ID string

	Type ElementType

	XMM      float64
	YMM      float64
	WidthMM  float64
	HeightMM float64

	Text *TextElement
}

type TextElement struct {
	Value    string
	FontSize float64
	Bold     bool
	Align    string
}
