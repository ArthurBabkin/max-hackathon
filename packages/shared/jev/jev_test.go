package jev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassify_SendsChoiceQuestionsAndReadsProbabilities(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/systemone" {
			t.Errorf("путь: %s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{
			"olympiad":{"type":"choice","choice":"p669-8","confidence":0.9,"probabilities":{"p669-8":0.9,"none":0.1}}},
			"usage":{"input_tokens":10,"output_tokens":2,"cost_rub":0.001}}`))
	}))
	defer srv.Close()

	c := New(srv.URL+"/api/v1/", "key")
	out, err := c.Classify(context.Background(), map[string]string{"message": "вышка по инфе"}, map[string]Choice{
		"olympiad": {Instructions: "О какой олимпиаде вопрос?", Classes: map[string]string{"p669-8": "Высшая проба", "none": "Нет"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["olympiad"]["p669-8"] != 0.9 || out["olympiad"]["none"] != 0.1 {
		t.Fatalf("вероятности: %v", out)
	}
	q := got["questions"].(map[string]any)["olympiad"].(map[string]any)
	if auth != "Bearer key" || got["model"] != "typesafe/jev" || got["state"].(map[string]any)["message"] != "вышка по инфе" ||
		q["type"] != "choice" || q["instructions"] != "О какой олимпиаде вопрос?" ||
		q["criteria"].(map[string]any)["p669-8"] != "Высшая проба" {
		t.Fatalf("запрос: %s %v", auth, got)
	}
}

func TestClassify_Errors(t *testing.T) {
	asked := map[string]Choice{"olympiad": {Classes: map[string]string{"a": "A"}}}
	for name, body := range map[string]string{
		"5xx":       "",
		"no answer": `{"answers":{}}`,
		"no probs":  `{"answers":{"olympiad":{"type":"choice","choice":"a","probabilities":{}}}}`,
		"not json":  `<html>`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if body == "" {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(body))
		}))
		if _, err := New(srv.URL, "k").Classify(context.Background(), nil, asked); err == nil {
			t.Errorf("%s: ждали ошибку", name)
		}
		srv.Close()
	}
	if New("x", "").Enabled() || !New("x", "k").Enabled() {
		t.Fatal("без ключа классификатор выключен, с ключом — включён")
	}
}
