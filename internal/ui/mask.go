package ui

import (
	"strings"

	"gioui.org/widget"
)

// maskDigits monta a data a partir só dos dígitos digitados, inserindo os
// separadores conforme eles vão sendo necessários: "05022026" vira
// "05/02/2026" e, com maxDigits 12, "050220261830" vira "05/02/2026 18:30".
// Um separador só aparece quando o dígito seguinte existe, para o backspace
// conseguir atravessá-lo naturalmente.
func maskDigits(s string, maxDigits int) string {
	var digits []rune
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
			if len(digits) == maxDigits {
				break
			}
		}
	}
	var b strings.Builder
	for i, d := range digits {
		switch i {
		case 2, 4:
			b.WriteByte('/')
		case 8:
			b.WriteByte(' ')
		case 10:
			b.WriteByte(':')
		}
		b.WriteRune(d)
	}
	return b.String()
}

// maskDateField reformata o campo a cada mudança, mantendo o cursor no fim.
// last guarda o texto do quadro anterior, para não mexer no editor à toa.
func maskDateField(ed *widget.Editor, last *string, maxDigits int) {
	txt := ed.Text()
	if txt == *last {
		return
	}
	masked := maskDigits(txt, maxDigits)
	if masked != txt {
		ed.SetText(masked)
		ed.SetCaret(len(masked), len(masked))
	}
	*last = ed.Text()
}
