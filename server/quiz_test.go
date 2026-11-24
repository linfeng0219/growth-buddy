package main

import (
	"math/rand"
	"strings"
	"testing"
)

func TestHardDifficultyAddsEnglishAndHigherMath(t *testing.T) {
	for _, age := range []int{4, 7} {
		normalQs := QueryOnlineQuestions(age, "normal", 24)
		for _, q := range normalQs {
			if q.Category == "英语" || q.Category == "数学拓展" {
				t.Fatalf("expected age=%d normal difficulty not to include %s question (%s)", age, q.Category, q.Question)
			}
		}

		hardQs := QueryOnlineQuestions(age, "hard", 24)
		hasEnglish := false
		hasHigherMath := false
		for _, q := range hardQs {
			if q.Category == "英语" {
				hasEnglish = true
			}
			if q.Category == "数学拓展" {
				hasHigherMath = true
			}
		}
		if !hasEnglish || !hasHigherMath {
			t.Fatalf("expected age=%d hard difficulty to include both English (%v) and higher-grade math (%v)", age, hasEnglish, hasHigherMath)
		}
	}
}

func TestCuratedBankIntegrity(t *testing.T) {
	seenIDs := map[string]bool{}
	seenQs := map[string]bool{}
	for _, q := range onlineCuratedBank {
		if q.ID == "" || seenIDs[q.ID] {
			t.Fatalf("invalid or duplicate ID: %q", q.ID)
		}
		seenIDs[q.ID] = true
		if q.Question == "" || seenQs[q.Question] {
			t.Fatalf("empty or duplicate question for %s: %q", q.ID, q.Question)
		}
		seenQs[q.Question] = true
		if len(q.Options) < 2 || q.AnswerIdx < 0 || q.AnswerIdx >= len(q.Options) {
			t.Fatalf("invalid options/answer for %s", q.ID)
		}
		optSet := map[string]bool{}
		for _, o := range q.Options {
			if o == "" || optSet[o] {
				t.Fatalf("duplicate/empty option in %s: %+v", q.ID, q.Options)
			}
			optSet[o] = true
		}
	}
	if len(onlineCuratedBank) < 730 {
		t.Fatalf("expected big curated bank >= 730 questions, got %d", len(onlineCuratedBank))
	}
}

func TestQuizExcludeAndNoDuplicateBatch(t *testing.T) {
	batch1 := QueryOnlineQuestionsWithExclude(7, "normal", 40, nil)
	if len(batch1) != 40 {
		t.Fatalf("expected 40 questions in batch1, got %d", len(batch1))
	}
	seenQ := map[string]bool{}
	exIDs := map[string]bool{}
	for _, q := range batch1 {
		if seenQ[q.Question] {
			t.Fatalf("duplicate question inside batch1: %q", q.Question)
		}
		seenQ[q.Question] = true
		if q.ID != "" {
			exIDs[q.ID] = true
		}
	}

	batch2 := QueryOnlineQuestionsWithExclude(7, "normal", 40, exIDs)
	if len(batch2) != 40 {
		t.Fatalf("expected 40 questions in batch2, got %d", len(batch2))
	}
	for _, q := range batch2 {
		if exIDs[q.ID] {
			t.Fatalf("batch2 returned excluded static ID %s", q.ID)
		}
	}
}

func TestTieredIdiomCoverageAndDynamicIntegrity(t *testing.T) {
	counts := map[string]int{}
	for _, q := range idiomCuratedBank {
		counts[q.AgeGroup+"|"+q.Difficulty]++
	}
	groups := []string{"3-5", "6-7", "8", "9-10", "11-14"}
	diffs := []string{"easy", "normal", "hard"}
	for _, g := range groups {
		for _, d := range diffs {
			if counts[g+"|"+d] < 8 {
				t.Fatalf("expected >= 8 static idiom questions for %s/%s, got %d", g, d, counts[g+"|"+d])
			}
		}
	}

	rng := rand.New(rand.NewSource(42))
	for _, g := range groups {
		for _, d := range diffs {
			qs := generateDynamicVerbalAndScienceQuestions(rng, g, d, 30)
			idiomCnt := 0
			for _, q := range qs {
				if strings.Contains(q.Question, "成语") {
					idiomCnt++
				}
				if len(q.Options) < 3 {
					t.Fatalf("dynamic question %s/%s has <3 options: %+v", g, d, q)
				}
				seenOpt := map[string]bool{}
				for _, o := range q.Options {
					if o == "" || seenOpt[o] {
						t.Fatalf("invalid/duplicate dynamic option for %s/%s (%s): %+v", g, d, q.Question, q.Options)
					}
					seenOpt[o] = true
				}
			}
			if idiomCnt < 4 {
				t.Fatalf("expected >=4 dynamic idiom questions in 30 verbal items for %s/%s, got %d", g, d, idiomCnt)
			}
		}
	}
}
