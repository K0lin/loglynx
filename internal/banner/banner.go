package banner

import (
	"fmt"

	"loglynx/internal/version"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

func Print() {
	ptermLogo, _ := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithRGB("Log", pterm.NewRGB(255, 107, 53)),
		putils.LettersFromStringWithRGB("Lynx", pterm.NewRGB(0, 0, 0))).
		Srender()

	pterm.DefaultCenter.Print(ptermLogo)

	pterm.DefaultCenter.Print(
		pterm.DefaultHeader.
			WithFullWidth().
			WithBackgroundStyle(pterm.NewStyle(pterm.BgLightRed)).
			WithMargin(5).
			Sprint(pterm.White("🐱 LogLynx - Fast & Precise Log Analytics")),
	)

	pterm.Info.Println(
		"Swift log monitoring and analytics. Fast like a lynx, precise like a predator." +
			"\nBuilt for speed and accuracy in production environments." +
			fmt.Sprintf("\nVersion %s.", version.Version),
	)
}
