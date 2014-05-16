package treewatcher

import (
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

var (
	tempDir string
)

func expectNoEvent(t *testing.T, tw *TreeWatcher) {
	time.Sleep(50 * time.Millisecond)
	for {
		select {
		case event := <-tw.Event:
			// Only fail if the error is for a file that exists. In some odd cases involving
			// move directories, we can get double notifications under the old and new directory names.
			_, err := os.Stat(event.Name)
			if err == nil {
				t.Error("Got unexpected event", event)
			} else {
				t.Log("Ignoring", event)
			}

		case err := <-tw.Error:
			t.Error("fswatcher returned error", err)

		default:
			return
		}
	}
}

func getEvent(t *testing.T, tw *TreeWatcher) *fsnotify.FileEvent {
	noError := false
	for !noError {
		select {
		case err := <-tw.Error:
			t.Error("fswatcher returned error", err)
		default:
			noError = true
		}
	}
	select {
	case event := <-tw.Event:
		t.Log(event)
		// This sleep takes care of race conditions in fsnotify so that the test can be more
		// strict about what it expects.
		time.Sleep(50 * time.Millisecond)
		return event
	// Wait at most 1 second for the event to show up.
	case <-time.After(1 * time.Second):
		return nil
	}

	return nil
}

func expectEvent(t *testing.T, tw *TreeWatcher) *fsnotify.FileEvent {
	event := getEvent(t, tw)
	if event == nil {
		t.Fatal("Expected an event but found none.")
	}
	return event
}

func writeFile(t *testing.T, tw *TreeWatcher, filename string, data string) {
	fullname := path.Join(tempDir, filename)
	_, err := os.Stat(fullname)
	create := err != nil

	var file *os.File
	if create {
		t.Log("Create file", fullname)
		file, err = os.Create(fullname)
		event := expectEvent(t, tw)
		if event.Name != fullname || !event.IsCreate() {
			t.Errorf("Expected create event on \"%s\" but got %s", filename, event.String())
		}
	} else {
		t.Log("Modify file", filename)
		file, err = os.OpenFile(fullname, os.O_WRONLY, 0666)
	}

	if err != nil {
		t.Fatal("Failed to open file", filename)
	}
	_, err = file.WriteString(data)
	if err != nil {
		t.Fatalf("Write file %s failed with error %s", filename, err)
	}
	err = file.Sync()
	if err != nil {
		t.Fatalf("Sync file %s failed with error %s", filename, err)
	}
	file.Close()

	event := expectEvent(t, tw)
	if event.Name != fullname || !event.IsModify() {
		t.Errorf("Expected modify event on \"%s\" but got %s", filename, event.String())
	}

	expectNoEvent(t, tw)
}

func mkDir(t *testing.T, tw *TreeWatcher, name string) {
	t.Log("Create directory", name)
	name = path.Join(tempDir, name)
	err := os.Mkdir(name, 0755)
	if err != nil {
		t.Fatalf("Mkdir %s failed with error %s", name, err)
	}
	event := expectEvent(t, tw)
	if !event.IsCreate() || event.Name != name {
		t.Errorf("Expected create event on \"%s\" but got %s", name, event.String())
	}

	expectNoEvent(t, tw)
}

func deleteFile(t *testing.T, tw *TreeWatcher, filename string) {
	t.Log("Remove file", filename)
	filename = path.Join(tempDir, filename)
	err := os.Remove(filename)
	if err != nil {
		t.Fatal("Remove file %s failed with error %s", filename, err)
	}
	event := expectEvent(t, tw)
	if !event.IsDelete() || event.Name != filename {
		t.Errorf("Expected create event on \"%s\" but got %s", filename, event.String())
	}
	expectNoEvent(t, tw)
}

func renameFile(t *testing.T, tw *TreeWatcher, oldName, newName string) {
	t.Logf("Rename %s to %s", oldName, newName)
	newName = path.Join(tempDir, newName)
	oldName = path.Join(tempDir, oldName)
	err := os.Rename(oldName, newName)
	if err != nil {
		t.Fatalf("Rename %s to %s failed with %s", oldName, newName, err)
	}

	events := make([]*fsnotify.FileEvent, 2)
	events[0] = expectEvent(t, tw)
	events[1] = expectEvent(t, tw)

	sawCreate := false
	sawRename := false

	for _, event := range events {
		if event.IsCreate() && event.Name == newName {
			sawCreate = true

		}

		if event.IsRename() && event.Name == oldName {
			sawRename = true
		}
	}

	if !sawCreate {
		t.Errorf("Expected create event on \"%s\" but got %v", newName, events)
	}

	if !sawRename {
		t.Errorf("Expected rename event on \"%s\" but got %v", oldName, events)
	}

	// Sometimes we get a 3rd event which is another rename. Sometimes not.
	getEvent(t, tw)

	expectNoEvent(t, tw)
}

func TestTreeWatcher(t *testing.T) {
	var err error
	tempDir, err = ioutil.TempDir("", "treewatcher_test")
	if err != nil {
		t.Fatal("Could not create temp directory")
	}
	t.Log("Using temporary directory", tempDir)
	// defer os.RemoveAll(tempDir)

	tw, err := New()
	if err != nil {
		t.Fatal("Could not create watcher:", err)
	}
	defer tw.Close()

	tw.WatchTree(tempDir)

	writeFile(t, tw, "abc.txt", "abc")
	writeFile(t, tw, "def.txt", "def")
	writeFile(t, tw, "abc.txt", "def")
	deleteFile(t, tw, "def.txt")
	renameFile(t, tw, "abc.txt", "def.txt")
	mkDir(t, tw, "dir")
	writeFile(t, tw, "dir/abc.txt", "abc")
	mkDir(t, tw, "dir2")
	renameFile(t, tw, "dir/abc.txt", "dir2/abc.txt")
	mkDir(t, tw, "dir/a")
	mkDir(t, tw, "dir/a/b")
	writeFile(t, tw, "dir/a/b/c.txt", "jklsdf")
	renameFile(t, tw, "dir/a", "dir2/a")
	writeFile(t, tw, "dir2/a/b/c.txt", "ggg")

	t.Log("Creating unwatched tree ppp/qqq/rrr/a.txt")
	unwatchedDir, _ := ioutil.TempDir("", "ppp")
	defer os.RemoveAll(unwatchedDir)

	nestedDir := path.Join(unwatchedDir, "ppp", "qqq", "rrr")
	os.MkdirAll(nestedDir, 0755)
	ioutil.WriteFile(path.Join(nestedDir, "a.txt"), []byte("data"), 0666)
	expectNoEvent(t, tw)

	t.Log("Moving unwatched tree under watched tree")
	movedDir := path.Join(tempDir, "ppp")
	os.Rename(path.Join(unwatchedDir, "ppp"), movedDir)
	event := expectEvent(t, tw)
	if event.Name != movedDir || !event.IsCreate() {
		t.Fatalf("Expected create of %s, got %s", movedDir, event)
	}
	writeFile(t, tw, "ppp/qqq/rrr/new.txt", "data")
	writeFile(t, tw, "ppp/qqq/rrr/a.txt", "data")
	deleteFile(t, tw, "ppp/qqq/rrr/new.txt")
	expectNoEvent(t, tw)
}
