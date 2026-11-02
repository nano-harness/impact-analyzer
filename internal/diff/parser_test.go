package diff

import (
	"reflect"
	"testing"
)

func TestParseUnifiedDiffRecognizesChangeKindsAndRenamePaths(t *testing.T) {
	diffText := `diff --git a/old.go b/new.go
similarity index 90%
rename from old.go
rename to new.go
--- a/old.go
+++ b/new.go
@@ -1,3 +1,4 @@
 package p
+func Added() {}
 func Kept() {}
-func Removed() {}
+func Changed() {}
`
	files, err := ParseUnifiedDiff(diffText)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files=%d, want 1", len(files))
	}
	if files[0].OldPath != "old.go" || files[0].NewPath != "new.go" {
		t.Fatalf("paths=(%q,%q), want old.go/new.go", files[0].OldPath, files[0].NewPath)
	}
	if len(files[0].Hunks) != 1 {
		t.Fatalf("hunks=%d, want 1", len(files[0].Hunks))
	}
}

func TestParseUnifiedDiffRenameWithoutHunks(t *testing.T) {
	files, err := ParseUnifiedDiff(`diff --git a/a.go b/a.go
similarity index 100%
rename from a.go
rename to b.go
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].OldPath != "a.go" || files[0].NewPath != "b.go" || len(files[0].Hunks) != 0 {
		t.Fatalf("files=%+v, want one pure rename file diff", files)
	}
}

func TestParseUnifiedDiffAddedAndDeletedFiles(t *testing.T) {
	files, err := ParseUnifiedDiff(`diff --git a/new.go b/new.go
new file mode 100644
--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+package p
+func New() {}
diff --git a/gone.go b/gone.go
deleted file mode 100644
--- a/gone.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package p
-func Gone() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files=%d, want 2", len(files))
	}
	if files[0].OldPath != "" || files[0].NewPath != "new.go" {
		t.Fatalf("added file paths=(%q,%q), want (\"\",\"new.go\")", files[0].OldPath, files[0].NewPath)
	}
	if files[1].OldPath != "gone.go" || files[1].NewPath != "" {
		t.Fatalf("deleted file paths=(%q,%q), want (\"gone.go\",\"\")", files[1].OldPath, files[1].NewPath)
	}
}

func TestHunkAffectedLines(t *testing.T) {
	files, err := ParseUnifiedDiff(`diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -10,3 +10,4 @@
 line
-old
+new
+added
 context
`)
	if err != nil {
		t.Fatal(err)
	}
	added, deleted := files[0].Hunks[0].AffectedLines()
	if !reflect.DeepEqual(added, []int{11, 12}) {
		t.Fatalf("added lines=%v, want [11 12]", added)
	}
	if len(deleted) != 1 || deleted[0] != 11 {
		t.Fatalf("deleted lines=%v, want [11]", deleted)
	}
}
