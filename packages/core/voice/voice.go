// Package voice — Go-двойник apps/web/src/voice: «ты» ученику, «вы»
// родителю (ТЗ F3). Бот, напоминания и уведомления говорят теми же словами,
// что и мини-приложение, потому что читают тот же словарь
// packages/shared/texts.
package voice

import (
	"fmt"
	"regexp"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/texts"
)

// Role — чей голос нужен.
type Role string

const (
	Kid    Role = "kid"
	Parent Role = "parent"
)

// Vars — значения подстановок. Числа приводятся к строке сами.
type Vars map[string]any

// KnownPlaceholders — разрешённые плейсхолдеры. Тот же список, что
// KNOWN_PLACEHOLDERS во фронте; тест сверяет оба и весь словарь.
var KnownPlaceholders = []string{
	"student",     // имя ученика, именительный падеж
	"student_gen", // родительный: «Цель Артёма», «В вузах Ольги»
	"student_dat", // дательный: «Предложить Артёму»
	"student_acc", // винительный: «Пригласить Артёма», «Пригласить Машу»
	"me",          // имя того, кто смотрит
	"name",        // имя третьего лица: кто отметил, кто предложил, кого удалили
	"creator",     // имя создателя траектории
	"names",       // перечисление имён через запятую
	"count",
	"total", // сколько всего: «Шаг 2 из 9»
	"date",
	"month",
	"direction",
	"grade",
	"grades", // классы через тире или запятую: «10–11», «9, 11»
	"from",   // границы диапазона: «от 75 до 90»
	"to",
	"region",
	"subject",
	"common", // с чем сравнивают: «…«Физика», а не «Астрономия»»
	"note",
	"stage",
	"title",
	"link", // ссылка-приглашение в сообщении бота
	"days", // «4 дня» — число со склонённым словом, собирает Go
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

// Interpolate подставляет {ключ}. Значения нет — плейсхолдер остаётся
// видимым: «{student}» в сообщении заметно при проверке, пустое место — нет.
func Interpolate(template string, vars Vars) string {
	return placeholder.ReplaceAllStringFunc(template, func(m string) string {
		if v, ok := vars[m[1:len(m)-1]]; ok {
			return fmt.Sprint(v)
		}
		return m
	})
}

// Text — строка словаря в голосе роли. Всё, что не Parent, говорит на «ты»,
// как во фронте. Неизвестный ключ возвращается как есть: ключ в сообщении
// заметен, а паника уронила бы весь ответ бота.
func Text(key string, role Role, vars Vars) string {
	t, ok := texts.Get(key)
	if !ok {
		return key
	}
	if role == Parent {
		return Interpolate(t.Parent, vars)
	}
	return Interpolate(t.Kid, vars)
}

// Voice — голос конкретного собеседника, как useVoice во фронте: падежи
// имени ученика посчитаны один раз и подставляются во все ключи сами.
type Voice struct {
	role Role
	base Vars
}

// New собирает голос: role — кто читает, student — имя ученика в
// именительном падеже, me — имя читающего.
func New(role Role, student, me string) Voice {
	return Voice{role: role, base: Vars{
		"student":     student,
		"student_gen": Genitive(student),
		"student_dat": Dative(student),
		"student_acc": Accusative(student),
		"me":          me,
	}}
}

// Role — чей это голос.
func (v Voice) Role() Role { return v.role }

// T — строка словаря; vars дополняют и переопределяют базовые подстановки.
func (v Voice) T(key string, vars Vars) string {
	if len(vars) == 0 {
		return Text(key, v.role, v.base)
	}
	merged := make(Vars, len(v.base)+len(vars))
	for k, val := range v.base {
		merged[k] = val
	}
	for k, val := range vars {
		merged[k] = val
	}
	return Text(key, v.role, merged)
}
