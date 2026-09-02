package label

type Document struct {
	WidthMM  float64
	HeightMM float64
	Shape    string

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
