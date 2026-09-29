package gitctx

import (
	"reflect"
	"testing"
)

func TestListFilesIsTrackedPlusUntrackedMinusIgnoredAndDeleted(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, ".gitignore", "build/\n")
	write(t, dir, "a.go", "package a\n")
	write(t, dir, "gone.go", "package a\n")
	commit(t, dir, "init")
	write(t, dir, "new_test.go", "package a\n")
	write(t, dir, "build/out.go", "package a\n")
	remove(t, dir, "gone.go")

	got, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "a.go", "new_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFiles = %v, want %v", got, want)
	}
}
