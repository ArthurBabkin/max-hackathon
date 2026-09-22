package voice

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/shared/texts"
)

// Кейсы text и interpolate — из apps/web/src/voice/dict.test.ts.

func TestText_ChoosesVariantByRole(t *testing.T) {
	if got := Text("home.goalLabel", Kid, nil); got != "Моя цель" {
		t.Errorf("kid: %q", got)
	}
	if got := Text("home.goalLabel", Parent, Vars{"student_gen": "Артёма"}); got != "Цель Артёма" {
		t.Errorf("parent: %q", got)
	}
}

func TestText_SubstitutesStudentAndViewer(t *testing.T) {
	if got := Text("home.greeting", Kid, Vars{"student": "Артём"}); got != "Привет, Артём 👋" {
		t.Errorf("kid: %q", got)
	}
	if got := Text("home.greeting", Parent, Vars{"me": "Ольга"}); got != "Здравствуйте, Ольга" {
		t.Errorf("parent: %q", got)
	}
}

func TestText_ProposesToParentInDative(t *testing.T) {
	if got := Text("olympiad.proposeCta", Parent, Vars{"student_dat": "Артёму"}); got != "Предложить Артёму" {
		t.Errorf("%q", got)
	}
}

func TestText_UnknownRoleFallsBackToKid(t *testing.T) {
	// Как во фронте: всё, что не parent, говорит на «ты».
	if got := Text("home.goalLabel", Role(""), nil); got != "Моя цель" {
		t.Errorf("%q", got)
	}
}

func TestText_UnknownKeyReturnsKey(t *testing.T) {
	// Как во фронте: ключ на экране заметен при вычитке, паника — нет.
	if got := Text("нет.такого", Parent, nil); got != "нет.такого" {
		t.Errorf("%q", got)
	}
}

func TestInterpolate(t *testing.T) {
	cases := []struct {
		tpl  string
		vars Vars
		want string
	}{
		{"{a} и {b}", Vars{"a": "раз", "b": "два"}, "раз и два"},
		{"выбрано: {count}", Vars{"count": 3}, "выбрано: 3"},
		{"{n}+{n}", Vars{"n": 1}, "1+1"},
		// без значения плейсхолдер остаётся видимым — так ошибку заметно
		{"Привет, {student}", Vars{}, "Привет, {student}"},
		{"Привет, {student}", nil, "Привет, {student}"},
	}
	for _, c := range cases {
		if got := Interpolate(c.tpl, c.vars); got != c.want {
			t.Errorf("Interpolate(%q) = %q, ждали %q", c.tpl, got, c.want)
		}
	}
}

func TestVoice_PrecomputesStudentCases(t *testing.T) {
	parent := New(Parent, "Артём", "Ольга")
	if got := parent.T("home.goalLabel", nil); got != "Цель Артёма" {
		t.Errorf("родительный падеж: %q", got)
	}
	if got := parent.T("olympiad.proposeCta", nil); got != "Предложить Артёму" {
		t.Errorf("дательный падеж: %q", got)
	}
	if got := parent.T("home.greeting", nil); got != "Здравствуйте, Ольга" {
		t.Errorf("имя смотрящего: %q", got)
	}
	if parent.Role() != Parent {
		t.Error("роль")
	}
	kid := New(Kid, "Артём", "Артём")
	if got := kid.T("home.greeting", nil); got != "Привет, Артём 👋" {
		t.Errorf("ученик: %q", got)
	}
	// Вызывающий может переопределить базовое значение.
	if got := kid.T("home.greeting", Vars{"student": "Тимур"}); got != "Привет, Тимур 👋" {
		t.Errorf("переопределение: %q", got)
	}
}

// Каждый плейсхолдер словаря известен — иначе на экране или в чате
// всплывёт сырое «{что-то}».
func TestDictionary_PlaceholdersKnown(t *testing.T) {
	re := regexp.MustCompile(`\{(\w+)\}`)
	for _, key := range texts.Keys() {
		v, _ := texts.Get(key)
		for _, s := range []string{v.Kid, v.Parent} {
			for _, m := range re.FindAllStringSubmatch(s, -1) {
				if !slices.Contains(KnownPlaceholders, m[1]) {
					t.Errorf("%s: неизвестный плейсхолдер {%s}", key, m[1])
				}
			}
		}
	}
}

// Список плейсхолдеров задан дважды — здесь и в apps/web/src/voice/texts.ts.
// Тест не даёт им разойтись.
func TestKnownPlaceholders_MatchFrontend(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "apps", "web", "src", "voice", "texts.ts"))
	if err != nil {
		t.Skipf("фронт недоступен: %v", err)
	}
	block := regexp.MustCompile(`(?s)KNOWN_PLACEHOLDERS = \[(.*?)\] as const`).FindSubmatch(src)
	if block == nil {
		t.Fatal("не нашёл KNOWN_PLACEHOLDERS в texts.ts")
	}
	var front []string
	for _, m := range regexp.MustCompile(`'(\w+)'`).FindAllSubmatch(block[1], -1) {
		front = append(front, string(m[1]))
	}
	if !slices.Equal(front, KnownPlaceholders) {
		t.Fatalf("списки разошлись:\nфронт: %v\nGo:    %v", front, KnownPlaceholders)
	}
}
