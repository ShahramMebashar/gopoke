package download

// Progress reports download/install progress.
type Progress struct {
	Tool          string  `json:"tool"`
	Stage         string  `json:"stage"`
	BytesReceived int64   `json:"bytesReceived"`
	BytesTotal    int64   `json:"bytesTotal"`
	Percent       float64 `json:"percent"`
	Message       string  `json:"message"`
}

// OnProgress is a callback for progress updates.
type OnProgress func(Progress)

func calcPercent(received, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(received) / float64(total) * 100
	if p > 100 {
		p = 100
	}
	return p
}
