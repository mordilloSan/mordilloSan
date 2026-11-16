// Package fonts embeds the typefaces the page is drawn with. All three are
// licensed under the SIL Open Font License; see the OFL-*.txt files.
package fonts

import "embed"

// FS holds the font files.
//
//go:embed *.ttf
var FS embed.FS
