package treesitter

import (
	"sync"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestParserPool_Get(t *testing.T) {
	parser1, err := GetParser(model.LanguageGo)
	if err != nil {
		t.Fatalf("GetParser failed: %v", err)
	}
	if parser1 == nil {
		t.Fatal("GetParser returned nil parser")
	}
	if parser1.lang != model.LanguageGo {
		t.Errorf("Expected Go parser, got %v", parser1.lang)
	}
}

func TestParserPool_PutAndReuse(t *testing.T) {
	// Get parser
	parser1, err := GetParser(model.LanguageJava)
	if err != nil {
		t.Fatalf("GetParser failed: %v", err)
	}

	// Return to pool
	PutParser(parser1)

	// Get again - should get same instance from pool
	parser2, err := GetParser(model.LanguageJava)
	if err != nil {
		t.Fatalf("GetParser failed: %v", err)
	}

	// Verify it's reused (same underlying parser object)
	if parser1.parser != parser2.parser {
		t.Log("Warning: Parser not reused (this is acceptable but suboptimal)")
	}
}

func TestParserPool_Concurrent(t *testing.T) {
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				parser, err := GetParser(model.LanguageJava)
				if err != nil {
					t.Errorf("GetParser failed: %v", err)
					return
				}
				// Do some work
				if parser.lang != model.LanguageJava {
					t.Errorf("Wrong parser language")
				}
				PutParser(parser)
			}
		}()
	}

	wg.Wait()
}

func TestWithParser(t *testing.T) {
	called := false
	err := WithParser(model.LanguagePython, func(p *Parser) error {
		called = true
		if p == nil {
			t.Error("Parser is nil")
		}
		if p.lang != model.LanguagePython {
			t.Errorf("Expected Python parser, got %v", p.lang)
		}
		return nil
	})

	if err != nil {
		t.Errorf("WithParser failed: %v", err)
	}
	if !called {
		t.Error("Function was not called")
	}
}

func TestWithParser_MultipleLanguages(t *testing.T) {
	languages := []model.Language{
		model.LanguageGo,
		model.LanguageJava,
		model.LanguageJavaScript,
		model.LanguageTypeScript,
		model.LanguagePython,
	}

	for _, lang := range languages {
		t.Run(string(lang), func(t *testing.T) {
			err := WithParser(lang, func(p *Parser) error {
				if p.lang != lang {
					t.Errorf("Expected %v parser, got %v", lang, p.lang)
				}
				return nil
			})
			if err != nil {
				t.Errorf("WithParser failed for %v: %v", lang, err)
			}
		})
	}
}
