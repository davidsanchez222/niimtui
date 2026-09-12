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

func NewQRElement(id, value string, xMM, yMM, sizeMM float64) Element {
	return Element{
		ID:       id,
		Type:     ElementQR,
		XMM:      xMM,
		YMM:      yMM,
		WidthMM:  sizeMM,
		HeightMM: sizeMM,
		QR: &QRElement{
			Value: value,
		},
	}
}
