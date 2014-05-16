treewatcher [![Build Status](https://travis-ci.org/dimfeld/treewatcher.png?branch=master)](https://travis-ci.org/dimfeld/treewatcher) [![GoDoc](http://godoc.org/github.com/dimfeld/treewatcher?status.png)](http://godoc.org/github.com/dimfeld/treewatcher)
===========

Recursive Filesystem Tree Watcher for Go

This is a wrapper around the fsnotify functionality from [howeyc/fsnotify](https://github.com/howeyc/fsnotify).

It exposes a similar interface, with an Event channel and an Error channel. All
data read from the fsnotify object is passed through to the client. 

When the TreeWatcher sees that a directory was added, it automatically starts
watching that directory and all directories under it.

## Usage

````go
tw = treewatcher.New()
defer tw.Close()
tw.WatchTree("/tmp")

for {
    select {
    event := <- tw.Event:
        handleEvent(event)
    error := <- tw.Error:
        panic(error)
    }
}
````

For a real usage example, see [dimfeld/simpleblog/fswatcher.go](https://github.com/dimfeld/simpleblog/blob/master/fswatcher.go).
