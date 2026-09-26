package store

import (
	"fmt"
	"slices"
	"testing"
)

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

// Разброс порога, предмет ЕГЭ и классы со 100 баллами у склеенной строки —
// одним предложением на все направления, как в сиде.
func TestMergeDirections(t *testing.T) {
	note := func(s string) *string { return &s }
	ege := func(n int) *int { return &n }
	got := mergeDirections([]BenefitRow{
		{Benefit: "bvi", EgeMin: ege(80), DiplomaGrades: []int32{11}, Varies: false,
			Note: note("Подтвердить ЕГЭ: Информатика или Математика")},
		{Benefit: "bvi", EgeMin: ege(75), DiplomaGrades: []int32{10, 11}, Varies: true,
			Note: note("Зависит от программы: на «А» льготы нет. За диплом 9 класса — 100 баллов. " +
				"Порог ЕГЭ зависит от программы: 75–85 баллов. Подтвердить ЕГЭ: Информатика или Физика")},
		{Benefit: "bvi", EgeMin: ege(75), DiplomaGrades: []int32{10, 11},
			Note: note("За диплом 10 класса — 100 баллов")},
	}, []string{"ИБ", "ПИ", "ИВТ"})
	want := "Зависит от программы (ПИ): на «А» льготы нет. За диплом 9 класса — 100 баллов. " +
		"Порог ЕГЭ зависит от программы: 75–85 баллов. Подтвердить ЕГЭ: Информатика или Математика или Физика"
	if got.Note == nil || *got.Note != want || *got.EgeMin != 75 || !got.Varies ||
		!slices.Equal(got.DiplomaGrades, []int32{10, 11}) {
		t.Fatalf("%+v\n%v", got, fmt.Sprint(*got.Note))
	}

	// Хоть у одного направления классы не ограничены — не ограничены и у
	// строки, оговорка про 100 баллов теряет смысл.
	open := mergeDirections([]BenefitRow{
		{Benefit: "bvi", DiplomaGrades: []int32{10, 11}, Note: note("За диплом 9 класса — 100 баллов")},
		{Benefit: "bvi"},
	}, []string{"ИБ", "ПИ"})
	if open.DiplomaGrades != nil || open.Note != nil || open.EgeMin != nil {
		t.Fatalf("классы без ограничений: %+v", open)
	}
}
