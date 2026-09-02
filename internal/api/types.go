package api

type Layout string

const (
	LayoutQROnly          Layout = "qr-only"
	LayoutQRTitle         Layout = "qr-title"
	LayoutQRTitleSubtitle Layout = "qr-title-subtitle"
)

type PrintRequest struct {
	Printer PrinterSelector `json:"printer"`
	Label   LabelRequest    `json:"label"`
	QR      QRRequest       `json:"qr,omitempty"`
	Content ContentRequest  `json:"content,omitempty"`
	Options PrintOptions    `json:"options,omitempty"`
}

type PrinterSelector struct {
	Selector string `json:"selector"`
}

type LabelRequest struct {
	Preset string `json:"preset"`
	Layout Layout `json:"layout,omitempty"`
}

type QRRequest struct {
	Text string `json:"text"`
}

type ContentRequest struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

type PrintOptions struct {
	Copies int `json:"copies,omitempty"`
}

type PrintResponse struct {
	OK      bool        `json:"ok"`
	Printer string      `json:"printer,omitempty"`
	Preset  string      `json:"preset,omitempty"`
	Copies  int         `json:"copies,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
