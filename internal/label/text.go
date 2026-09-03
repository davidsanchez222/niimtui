package label

func NewTextElement(id, value string, xMM, yMM, widthMM, heightMM, fontSize float64) Element {
	return Element{
		ID:       id,
		Type:     ElementText,
		XMM:      xMM,
		YMM:      yMM,
		WidthMM:  widthMM,
		HeightMM: heightMM,
		Text: &TextElement{
			Value:    value,
			FontSize: fontSize,
		},
	}
}
