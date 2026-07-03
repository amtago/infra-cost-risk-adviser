package tags

import (
	"os"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "required-tags-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

func TestLoadFile_Basic(t *testing.T) {
	path := writeTemp(t, "Env\nTeam\nCostCenter\n")
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Env", "Team", "CostCenter"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: want %s, got %s", i, want[i], got[i])
		}
	}
}

func TestLoadFile_CommentsAndBlanks(t *testing.T) {
	path := writeTemp(t, "# required tags\nEnv\n\n# another comment\nTeam\n")
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "Env" || got[1] != "Team" {
		t.Errorf("expected [Env Team], got %v", got)
	}
}

func TestLoadFile_Empty(t *testing.T) {
	path := writeTemp(t, "# just comments\n\n")
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/does/not/exist.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFile_Whitespace(t *testing.T) {
	path := writeTemp(t, "  Env  \n  Team  \n")
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "Env" || got[1] != "Team" {
		t.Errorf("whitespace not trimmed, got %v", got)
	}
}
