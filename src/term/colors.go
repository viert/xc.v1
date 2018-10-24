package term

import (
	"fmt"
)

type colorValue int

const (
	CBlack        colorValue = 30
	CRed          colorValue = 31
	CGreen        colorValue = 32
	CYellow       colorValue = 33
	CBlue         colorValue = 34
	CMagenta      colorValue = 35
	CCyan         colorValue = 36
	CLightGray    colorValue = 37
	CDarkGray     colorValue = 90
	CLightRed     colorValue = 91
	CLightGreen   colorValue = 92
	CLightYellow  colorValue = 93
	CLightBlue    colorValue = 94
	CLightMagenta colorValue = 95
	CLightCyan    colorValue = 96
	CWhite        colorValue = 97
)

func Colored(msg string, c colorValue, bold bool) string {
	bstr := ""
	if bold {
		bstr = ";1"
	}
	return fmt.Sprintf("\033[%d%sm%s\033[0m", c, bstr, msg)
}

func Blue(msg string) string {
	return Colored(msg, CLightBlue, false)
}

func Red(msg string) string {
	return Colored(msg, CLightRed, false)
}

func Green(msg string) string {
	return Colored(msg, CLightGreen, false)
}

func Yellow(msg string) string {
	return Colored(msg, CLightYellow, false)
}

func Cyan(msg string) string {
	return Colored(msg, CLightCyan, false)
}

func Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Print(Red(msg))
}

func Successf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Print(Green(msg))
}

func Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Print(Yellow(msg))
}
