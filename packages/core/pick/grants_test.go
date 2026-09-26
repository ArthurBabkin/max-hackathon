package pick

import (
	"fmt"
	"testing"

	"github.com/ArthurBabkin/max-hackathon/packages/db/store"
)

func TestGrants_WinnerAndPrizerFromBenefitAndNote(t *testing.T) {
	note := func(s string) *string { return &s }
	for _, tc := range []struct {
		name           string
		row            store.BenefitRow
		winner, prizer string
	}{
		{"БВИ обоим", store.BenefitRow{Benefit: "bvi"}, "bvi", "bvi"},
		{"БВИ только победителю", store.BenefitRow{Benefit: "bvi", Note: note("БВИ только победителю. Подтвердить ЕГЭ: Физика")}, "bvi", ""},
		{"победителю БВИ, призёру 100", store.BenefitRow{Benefit: "bvi_winners", Note: note("Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Физика")}, "bvi", "score100"},
		{"БВИ победителям без призёров", store.BenefitRow{Benefit: "bvi_winners"}, "bvi", ""},
		{"100 баллов обоим", store.BenefitRow{Benefit: "score100"}, "score100", "score100"},
		{"100 баллов только победителю", store.BenefitRow{Benefit: "score100", Note: note("100 баллов только победителю.")}, "score100", ""},
		{"доп. баллы", store.BenefitRow{Benefit: "extra_points"}, "extra_points", "extra_points"},
		{"неизвестная льгота", store.BenefitRow{Benefit: "x"}, "", ""},
	} {
		if w, p := Grants(tc.row); w != tc.winner || p != tc.prizer {
			t.Errorf("%s: %q/%q, ждали %q/%q", tc.name, w, p, tc.winner, tc.prizer)
		}
	}
}

func TestNoteSentences(t *testing.T) {
	s := "Победителю — БВИ, призёру — 100 баллов. Подтвердить ЕГЭ: Физика."
	got := NoteSentences(&s)
	if len(got) != 2 || got[0] != "Победителю — БВИ, призёру — 100 баллов" || got[1] != "Подтвердить ЕГЭ: Физика" {
		t.Fatalf("%q", got)
	}
	if NoteSentences(nil) != nil {
		t.Fatal("нет примечания — нет предложений")
	}

	// Точка внутри скобок и кавычек — сокращение, а не конец предложения.
	s = "Зависит от программы: льгота только на «Экономика» (Филиал МГУ в г. Севастополе). Подтвердить ЕГЭ: Математика"
	got = NoteSentences(&s)
	if len(got) != 2 || got[0] != "Зависит от программы: льгота только на «Экономика» (Филиал МГУ в г. Севастополе)" {
		t.Fatalf("%q", got)
	}
}

// Сид (#78): классы строки — у лучшей льготы, классы со 100 баллами — в
// примечании «За диплом 9 класса — 100 баллов».
func TestScore100Grades(t *testing.T) {
	for note, want := range map[string]string{
		"Подтвердить ЕГЭ: Информатика. За диплом 9 класса — 100 баллов":                    "За диплом 9 класса — 100 баллов [9]",
		"За диплом 9–10 класса — 100 баллов. Порог ЕГЭ зависит от программы: 75–85 баллов": "За диплом 9–10 класса — 100 баллов [9 10]",
		"За диплом 9, 11 класса — 100 баллов":                                              "За диплом 9, 11 класса — 100 баллов [9 11]",
		"Подтвердить ЕГЭ: Информатика":                                                     " []",
		"За диплом какого-то класса — 100 баллов":                                          " []",
	} {
		s, grades := Score100Grades(&note)
		if got := fmt.Sprintf("%s %v", s, grades); got != want {
			t.Errorf("%q: %q, ждали %q", note, got, want)
		}
	}
	if s, g := Score100Grades(nil); s != "" || g != nil {
		t.Fatal("нет примечания — нет оговорки")
	}
}

func TestNick(t *testing.T) {
	if Nick("innopolis", "УИ") != "Иннополис" || Nick("itmo", "ИТМО") != "ИТМО" {
		t.Fatal("аббревиатуру, которая ничего не скажет школьнику, заменяем привычным названием")
	}
}
