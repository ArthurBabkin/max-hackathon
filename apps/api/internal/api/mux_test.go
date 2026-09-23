package api

import "testing"

// Под нашим go.mod роутер новый. Старый режим воспроизводится запуском с
// GODEBUG=httpmuxgo121=1 — тогда CheckMux обязан вернуть ошибку.
func TestCheckMux(t *testing.T) {
	if err := CheckMux(); err != nil {
		t.Fatal(err)
	}
}
