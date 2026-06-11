package vpn

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// renderQRPNG encodes content as a PNG QR code at the given pixel size.
func renderQRPNG(content string, size int) ([]byte, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}
	return png, nil
}
