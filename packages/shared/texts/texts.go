// Package texts — Go-загрузчик общего словаря texts.json. Его же читает
// мини-приложение (apps/web/src/voice/texts.ts), поэтому правило «ты» ученику
// и «вы» родителю (ТЗ F3) одинаково в чате и на экранах.
//
// Загрузчик лежит рядом с файлом, потому что go:embed не выходит за каталог
// пакета. Выбор варианта и подстановки — в packages/core/voice.
package texts

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

// Text — два варианта одной строки.
type Text struct {
	Kid    string `json:"kid"`
	Parent string `json:"parent"`
}

//go:embed texts.json
var textsJSON []byte

var (
	dict map[string]Text
	keys []string
)

func init() {
	var err error
	if dict, err = parse(textsJSON); err != nil {
		panic("texts: " + err.Error())
	}
	keys = make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}
	sort.Strings(keys)
}

// parse строго разбирает словарь: опечатка в имени варианта («parnt»)
// превратилась бы в пустую строку на экране, поэтому это ошибка.
func parse(raw []byte) (map[string]Text, error) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("битый texts.json: %w", err)
	}
	out := make(map[string]Text, len(entries))
	for key, body := range entries {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		var t Text
		if err := dec.Decode(&t); err != nil {
			return nil, fmt.Errorf("ключ %s: %w", key, err)
		}
		out[key] = t
	}
	return out, nil
}

// Get возвращает оба варианта ключа.
func Get(key string) (Text, bool) {
	t, ok := dict[key]
	return t, ok
}

// Keys — все ключи словаря по алфавиту. Возвращает копию.
func Keys() []string {
	return append([]string(nil), keys...)
}
