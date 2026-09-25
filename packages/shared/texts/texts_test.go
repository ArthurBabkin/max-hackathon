package texts

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Те же инварианты, что проверяет apps/web/src/voice/dict.test.ts: словарь
// общий, и Go-сборка не должна полагаться на то, что CI фронта зелёный.

func TestDictionary_NotEmpty(t *testing.T) {
	if n := len(Keys()); n < 100 {
		t.Fatalf("в словаре %d ключей — похоже, файл не прочитался", n)
	}
}

func TestDictionary_EveryKeyHasBothVariants(t *testing.T) {
	for _, key := range Keys() {
		v, _ := Get(key)
		if strings.TrimSpace(v.Kid) == "" || strings.TrimSpace(v.Parent) == "" {
			t.Errorf("%s: пустой вариант kid=%q parent=%q", key, v.Kid, v.Parent)
		}
	}
}

func TestDictionary_NoMarkup(t *testing.T) {
	tag := regexp.MustCompile(`(?i)<[a-z/]`)
	for _, key := range Keys() {
		v, _ := Get(key)
		if tag.MatchString(v.Kid) || tag.MatchString(v.Parent) {
			t.Errorf("%s: в тексте разметка — бот отправляет его как есть", key)
		}
	}
}

// Формулировки нейтральны по роду: «Я зарегистрировался» в тексте для
// ученицы звучит чужим голосом. Вместо формы глагола — действие: «отметить
// регистрацию».
func TestDictionary_NoGenderedFirstPerson(t *testing.T) {
	gendered := regexp.MustCompile(`(?i)(^|[^а-яё])я\s+(уже\s+)?[а-яё]+(лся|лась)([^а-яё]|$)`)
	for _, key := range Keys() {
		v, _ := Get(key)
		if gendered.MatchString(v.Kid) || gendered.MatchString(v.Parent) {
			t.Errorf("%s: глагол в роде от первого лица — нужна нейтральная формулировка", key)
		}
	}
}

func TestParse_RejectsTyposInVariantNames(t *testing.T) {
	if _, err := parse([]byte(`{"a.b": {"kid": "ты", "parnt": "вы"}}`)); err == nil {
		t.Fatal("опечатка в имени варианта должна ронять разбор, а не давать пустую строку")
	}
	if _, err := parse([]byte(`{"a.b": {"kid": "ты", "parent": "вы"}}`)); err != nil {
		t.Fatalf("валидный словарь не разобрался: %v", err)
	}
}

func TestGet(t *testing.T) {
	v, ok := Get("home.goalLabel")
	if !ok || v.Kid != "Моя цель" || v.Parent != "Цель {student_gen}" {
		t.Fatalf("Get(home.goalLabel) = %+v, %v", v, ok)
	}
	if _, ok := Get("нет.такого"); ok {
		t.Fatal("неизвестный ключ — ok=false")
	}
}

func TestKeys_SortedCopy(t *testing.T) {
	keys := Keys()
	if !sort.StringsAreSorted(keys) {
		t.Fatal("Keys отсортированы — тесты и отчёты детерминированы")
	}
	keys[0] = "испорчено"
	if Keys()[0] == "испорчено" {
		t.Fatal("Keys отдаёт копию")
	}
}
