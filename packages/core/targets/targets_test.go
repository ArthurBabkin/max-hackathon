package targets

import (
	"slices"
	"testing"
)

func TestCovers_EqualCodes(t *testing.T) {
	if !Covers("09.03.04", "09.03.04") {
		t.Fatal("одинаковые коды должны покрывать друг друга")
	}
	if Covers("09.03.04", "09.03.01") {
		t.Fatal("разные направления одной группы не покрывают друг друга")
	}
}

// Иннополис даёт укрупнённую группу 09.00.00, а ученик выбирает 09.03.04.
func TestCovers_EnlargedGroupBothWays(t *testing.T) {
	if !Covers("09.00.00", "09.03.04") {
		t.Fatal("09.00.00 покрывает 09.03.04")
	}
	if !Covers("09.03.04", "09.00.00") {
		t.Fatal("09.03.04 покрывает 09.00.00")
	}
}

func TestCovers_EnlargedGroupOnlyItsOwn(t *testing.T) {
	if Covers("09.00.00", "10.03.01") || Covers("10.03.01", "09.00.00") {
		t.Fatal("09.00.00 не покрывает 10.03.01")
	}
	if Covers("09.00.00", "10.00.00") {
		t.Fatal("разные укрупнённые группы не покрывают друг друга")
	}
}

func TestCovers_EmptyCodeCoversNothing(t *testing.T) {
	if Covers("", "") || Covers("", "09.03.04") || Covers("09.00.00", "") {
		t.Fatal("пустой код ничего не покрывает")
	}
}

// Направления МФТИ: одно проверено, одно ждёт проверки.
var mipt = []Offered{
	{DirectionID: "napr-01-03-02", Code: "01.03.02", Status: "offered"},
	{DirectionID: "napr-03-03-01", Code: "03.03.01", Status: "offered"},
	{DirectionID: "napr-09-03-01", Code: "09.03.01", Status: "offered"},
	{DirectionID: "napr-10-03-01", Code: "10.03.01", Status: "to_check"},
}

func TestResolve_ChosenBeatsGoal(t *testing.T) {
	got := Resolve("mipt", mipt, []string{"napr-09-03-01", "napr-01-03-02"}, []string{"03.03.01"})
	if got.Basis != Chosen {
		t.Fatalf("основание %q, ожидали chosen", got.Basis)
	}
	// Порядок — как у направлений вуза, а не как выбирал ученик.
	if !slices.Equal(got.DirectionIDs, []string{"napr-01-03-02", "napr-09-03-01"}) {
		t.Fatalf("направления %v", got.DirectionIDs)
	}
	if got.UniversityID != "mipt" || got.Unverified {
		t.Fatalf("цель %+v", got)
	}
}

func TestResolve_ChosenOutsideUniversityIsIgnored(t *testing.T) {
	got := Resolve("mipt", mipt, []string{"napr-31-05-01", "napr-03-03-01"}, nil)
	if got.Basis != Chosen || !slices.Equal(got.DirectionIDs, []string{"napr-03-03-01"}) {
		t.Fatalf("цель %+v", got)
	}
}

// Выбор устарел: у вуза больше нет этих направлений — дальше как без выбора.
func TestResolve_ChosenAllOutsideFallsBackToGoal(t *testing.T) {
	got := Resolve("mipt", mipt, []string{"napr-31-05-01"}, []string{"09.03.01"})
	if got.Basis != Goal || !slices.Equal(got.DirectionIDs, []string{"napr-09-03-01"}) {
		t.Fatalf("цель %+v", got)
	}
}

func TestResolve_GoalCodesPickUniversityDirections(t *testing.T) {
	got := Resolve("mipt", mipt, nil, []string{"10.03.01", "01.03.02", "38.03.01"})
	if got.Basis != Goal {
		t.Fatalf("основание %q, ожидали goal", got.Basis)
	}
	if !slices.Equal(got.DirectionIDs, []string{"napr-01-03-02", "napr-10-03-01"}) {
		t.Fatalf("направления %v", got.DirectionIDs)
	}
	if got.Unverified {
		t.Fatal("01.03.02 проверено — цель не «уточняется»")
	}
}

// Иннополис: у вуза одна укрупнённая группа, ученик хочет 09.03.04.
func TestResolve_GoalCoveredByEnlargedGroup(t *testing.T) {
	innopolis := []Offered{{DirectionID: "napr-09-00-00", Code: "09.00.00", Status: "offered"}}
	got := Resolve("innopolis", innopolis, nil, []string{"09.03.04"})
	if got.Basis != Goal || !slices.Equal(got.DirectionIDs, []string{"napr-09-00-00"}) {
		t.Fatalf("цель %+v", got)
	}
}

func TestResolve_GoalNotCoveredMeansWholeUniversity(t *testing.T) {
	got := Resolve("mipt", mipt, nil, []string{"31.05.01"})
	if got.Basis != University {
		t.Fatalf("основание %q, ожидали university", got.Basis)
	}
	if len(got.DirectionIDs) != 0 || got.Unverified {
		t.Fatalf("цель %+v", got)
	}
}

func TestResolve_NoChoiceNoGoalMeansWholeUniversity(t *testing.T) {
	if got := Resolve("mipt", mipt, nil, nil); got.Basis != University {
		t.Fatalf("основание %q, ожидали university", got.Basis)
	}
	if got := Resolve("mipt", nil, []string{"napr-09-03-01"}, []string{"09.03.01"}); got.Basis != University {
		t.Fatalf("у вуза нет направлений — основание %q, ожидали university", got.Basis)
	}
}

// Все направления цели ждут проверки: льготы по ним «уточняются».
func TestResolve_UnverifiedWhenAllTargetDirectionsToCheck(t *testing.T) {
	got := Resolve("mipt", mipt, nil, []string{"10.03.01"})
	if got.Basis != Goal || !got.Unverified {
		t.Fatalf("цель %+v", got)
	}
	got = Resolve("mipt", mipt, []string{"napr-10-03-01"}, nil)
	if got.Basis != Chosen || !got.Unverified {
		t.Fatalf("цель %+v", got)
	}
	got = Resolve("mipt", mipt, []string{"napr-10-03-01", "napr-09-03-01"}, nil)
	if got.Unverified {
		t.Fatal("одно из направлений проверено — цель не «уточняется»")
	}
}
