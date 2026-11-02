package lang_test

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/lang"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestAllAdaptersRegistered(t *testing.T) {
	for _, l := range []model.Language{
		model.LanguageGo,
		model.LanguageJava,
		model.LanguageJavaScript,
		model.LanguageTypeScript,
	} {
		if !lang.IsRegistered(l) {
			t.Errorf("language %s adapter not registered", l)
		}
	}
}
