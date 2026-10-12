package label

type Document struct {
	WidthMM  float64
	HeightMM float64
	Shape    string
	Rotation int
	Inverted bool

	Elements []Element
}

func NewDocument(widthMM, heightMM float64) Document {
	return Document{
		WidthMM:  widthMM,
		HeightMM: heightMM,
		Shape:    "rect",
		Elements: []Element{},
	}
}

// PhysicalDimensionsMM returns the roll dimensions independently of the editing orientation.
func (d Document) PhysicalDimensionsMM() (float64, float64) {
	rotation := (d.Rotation%360 + 360) % 360
	if rotation == 90 || rotation == 270 {
		return d.HeightMM, d.WidthMM
	}
	return d.WidthMM, d.HeightMM
}
