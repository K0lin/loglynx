// MIT License
//
// Copyright (c) 2026 Kolin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
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

