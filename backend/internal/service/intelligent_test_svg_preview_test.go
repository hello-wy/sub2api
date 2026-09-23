package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntelligentSVGPreviewSupportsLengthsAndImplicitPaths(t *testing.T) {
	for name, shape := range map[string]string{
		"pixel circle":       `<circle cx="50" cy="50" r="40px"/>`,
		"pixel rectangle":    `<rect width="80px" height="50px"/>`,
		"pixel ellipse":      `<ellipse cx="50" cy="50" rx="40px" ry="20px"/>`,
		"percent rectangle":  `<rect width="80%" height="50%"/>`,
		"absolute length":    `<circle cx="50" cy="50" r="1cm"/>`,
		"exponent length":    `<circle cx="50" cy="50" r="4e1px"/>`,
		"implicit absolute":  `<path d="M0 0 100 0 100 100 0 100z"/>`,
		"implicit relative":  `<path d="m0,0 100,0 0,100 -100,0z"/>`,
		"adjacent signs":     `<path d="M50 50-40-40 80 0z"/>`,
		"exponent path":      `<path d="M0 0 1e2 0 1e2 1e2z"/>`,
		"explicit curves":    `<path d="M0 0Q50 100 100 0"/>`,
		"adjacent arc flags": `<path d="M0 0 A10 10 0 0110 20"/>`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := `<svg viewBox="0 0 100 100">` + shape + `</svg>`
			result := evaluateIntelligentSVG(raw)
			require.Equal(t, "completed", result.Status)
			require.Equal(t, "ready", result.Detail["image_state"])
			require.Equal(t, IntelligentSVGPreviewVersion, result.Detail["image_preview_version"])
			require.NotEmpty(t, result.Image)
			require.Nil(t, result.Score)
			_, err := SanitizeIntelligentTestSVG(result.Image)
			require.NoError(t, err)
		})
	}
}

func TestIntelligentSVGPreviewStillRejectsNonDrawings(t *testing.T) {
	for _, shape := range []string{
		`<circle r="0px"/>`, `<circle r="-4px"/>`, `<circle r="40invalid"/>`,
		`<circle r="+Inf"/>`, `<circle r="1e999px"/>`, `<circle r="NaN"/>`,
		`<rect width="40px" height="0%"/>`, `<path d="M0 0Z"/>`,
		`<path d="M0 0M10 10"/>`, `<path d="M0 0 10"/>`,
		`<path d="L10 10"/>`, `<path d="M0 0 L"/>`,
		`<path d="M0 0Q10 10"/>`, `<path d="M0 0A10 10 0 2 1 20 20"/>`,
	} {
		t.Run(shape, func(t *testing.T) {
			result := evaluateIntelligentSVG(`<svg>` + shape + `</svg>`)
			require.Empty(t, result.Image)
			require.Equal(t, "unavailable", result.Detail["image_state"])
			require.Equal(t, IntelligentSVGPreviewVersion, result.Detail["image_preview_version"])
		})
	}
}
