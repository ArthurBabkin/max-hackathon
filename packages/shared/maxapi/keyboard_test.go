package maxapi

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// MAX отвергает сообщение целиком, если текст длиннее MaxTextLen, поэтому
// лимит проверяется в символах, а не в байтах: кириллица занимает по два.
func TestTextTruncatesToLimit(t *testing.T) {
	long := strings.Repeat("я", MaxTextLen+500)
	m := Text(long)
	if n := utf8.RuneCountInString(m.Text); n != MaxTextLen {
		t.Fatalf("длина %d символов, ожидали %d", n, MaxTextLen)
	}
	if !strings.HasSuffix(m.Text, "…") {
		t.Error("обрыв текста не помечен многоточием")
	}
	if len(m.Text) <= MaxTextLen {
		t.Error("тест бессмысленный: текст уложился в лимит и по байтам")
	}
}

func TestTextKeepsShortTextAsIs(t *testing.T) {
	for _, s := range []string{"", "привет", strings.Repeat("я", MaxTextLen)} {
		if got := Text(s).Text; got != s {
			t.Errorf("текст длиной %d символов изменён", utf8.RuneCountInString(s))
		}
	}
}

func TestWithKeyboardTruncatesText(t *testing.T) {
	m := WithKeyboard(strings.Repeat("a", MaxTextLen+1), Keyboard{{{Type: "callback", Text: "ок"}}})
	if n := utf8.RuneCountInString(m.Text); n != MaxTextLen {
		t.Fatalf("длина %d символов, ожидали %d", n, MaxTextLen)
	}
	if len(m.Keyboard()) != 1 {
		t.Error("клавиатура потерялась при обрезке")
	}
}

// Обрыв не должен оставлять висящий пробел перед многоточием.
func TestTruncateTrimsTrailingSpace(t *testing.T) {
	if got := Truncate("аб вгд", 4); got != "аб…" {
		t.Errorf("получили %q, ожидали %q", got, "аб…")
	}
}
